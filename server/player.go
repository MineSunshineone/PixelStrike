package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/gorilla/websocket"
)

const (
	writeChanSize      = 64
	writeDeadlineEvery = 30
	maxMsgSize         = 4096
	readDeadline       = 75 * time.Second
	pingPeriod         = 20 * time.Second
)

var upgrader = websocket.Upgrader{ReadBufferSize: 2048, WriteBufferSize: 2048, CheckOrigin: sameOrigin}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

type IPResolver []netip.Prefix

func NewIPResolver(value string) (IPResolver, error) {
	var resolver IPResolver
	for _, raw := range strings.Split(value, ",") {
		raw = strings.TrimSpace(raw)
		if raw == "" {
			continue
		}
		prefix, err := netip.ParsePrefix(raw)
		if err != nil {
			if address, addressErr := netip.ParseAddr(raw); addressErr == nil {
				address = address.Unmap()
				prefix = netip.PrefixFrom(address, address.BitLen())
			} else {
				return nil, fmt.Errorf("invalid trusted proxy %q: %w", raw, err)
			}
		}
		resolver = append(resolver, prefix.Masked())
	}
	return resolver, nil
}

func (resolver IPResolver) ClientIP(r *http.Request) string {
	peer, ok := parseRequestIP(r.RemoteAddr)
	if !ok {
		return ""
	}
	if !resolver.trusted(peer) {
		return peer.String()
	}
	forwarded := strings.Split(r.Header.Get("X-Forwarded-For"), ",")
	for i := len(forwarded) - 1; i >= 0; i-- {
		if address, valid := parseRequestIP(forwarded[i]); valid && !resolver.trusted(address) {
			return address.String()
		}
	}
	if address, valid := parseRequestIP(r.Header.Get("X-Real-IP")); valid && !resolver.trusted(address) {
		return address.String()
	}
	return peer.String()
}

func (resolver IPResolver) trusted(address netip.Addr) bool {
	for _, prefix := range resolver {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func parseRequestIP(value string) (netip.Addr, bool) {
	value = strings.TrimSpace(value)
	if addressPort, err := netip.ParseAddrPort(value); err == nil {
		address := addressPort.Addr()
		return address.Unmap(), address.Zone() == ""
	}
	address, err := netip.ParseAddr(strings.Trim(value, "[]"))
	if err != nil || address.Zone() != "" {
		return netip.Addr{}, false
	}
	return address.Unmap(), true
}

type Player struct {
	PlayerState
	ws                    *websocket.Conn
	send                  chan []byte
	latestSnapshot        chan []byte
	snapshotBuffers       chan []byte
	sendMu                sync.RWMutex
	inputMu               sync.Mutex
	closeOnce             sync.Once
	Room                  *Room
	Hub                   *Hub
	IP, Fingerprint       string
	joined, ready, closed bool
	netCache              map[uint16]quantState
	netFullAt             map[uint16]uint32
	lastSelf              compactSelfState
	hasLastSelf           bool
	rosterRequested       bool
	queuedInput           playerInput
	hasQueuedInput        bool
	netReset              atomic.Bool
}

type playerInput struct {
	seq        uint16
	keys       uint8
	yaw, pitch float64
	at         time.Time
}

func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request, allowedOrigin string, ips IPResolver) {
	connectionUpgrader := upgrader
	connectionUpgrader.CheckOrigin = func(request *http.Request) bool {
		if allowedOrigin != "" {
			return request.Header.Get("Origin") == allowedOrigin
		}
		return sameOrigin(request)
	}
	ip := ips.ClientIP(r)
	if ip == "" {
		http.Error(w, "invalid client address", http.StatusBadRequest)
		return
	}
	if hub.Store.IsIPBanned(ip) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	conn, err := connectionUpgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	p := &Player{
		ws: conn, send: make(chan []byte, writeChanSize), latestSnapshot: make(chan []byte, 1), snapshotBuffers: make(chan []byte, 2),
		Hub: hub, IP: ip,
	}
	p.ApplyLoadout(3, 0)
	p.HP = MaxHP
	p.Armor = 100
	p.Grenades = 1
	go p.writePump()
	p.readPump(hub)
}

func (p *Player) readPump(hub *Hub) {
	defer hub.Leave(p)
	p.ws.SetReadLimit(maxMsgSize)
	p.ws.SetReadDeadline(time.Now().Add(readDeadline))
	p.ws.SetPongHandler(func(string) error {
		return p.ws.SetReadDeadline(time.Now().Add(readDeadline))
	})
	var readBuffer bytes.Buffer
	readBuffer.Grow(maxMsgSize)
	for {
		mt, reader, err := p.ws.NextReader()
		if err != nil {
			return
		}
		readBuffer.Reset()
		if _, err := readBuffer.ReadFrom(reader); err != nil {
			return
		}
		data := readBuffer.Bytes()
		if mt != websocket.BinaryMessage || len(data) < 1 {
			continue
		}
		p.ws.SetReadDeadline(time.Now().Add(readDeadline))
		op, payload := data[0], data[1:]
		now := time.Now()
		if p.joined && op != OpPing && !p.InputRateOK(now) {
			log.Printf("player %d message flood", p.Id)
			return
		}
		switch op {
		case OpJoin:
			if p.joined || len(payload) < 7 {
				return
			}
			if hub.Store.IsIPBanned(p.IP) {
				p.Send(Reject("访问已被封禁"))
				time.Sleep(20 * time.Millisecond)
				return
			}
			if payload[0] != ProtocolVersion {
				p.Send(Reject("版本已更新，请刷新页面"))
				time.Sleep(50 * time.Millisecond)
				return
			}
			n := int(payload[1])
			if n > len(payload)-7 {
				continue
			}
			name := sanitizeName(string(payload[2 : 2+n]))
			primary, secondary, skin := payload[2+n], payload[3+n], payload[4+n]
			primaryWeaponSkin, secondaryWeaponSkin := payload[5+n], payload[6+n]
			if !validLoadout(primary, secondary) {
				primary, secondary = 3, 0
			}
			if skin >= SkinCount {
				skin = 0
			}
			if len(payload) > 7+n {
				p.Fingerprint = sanitizeFingerprint(string(payload[7+n:]))
			}
			if name == "" {
				p.Send(Reject("请输入玩家名字"))
				time.Sleep(20 * time.Millisecond)
				return
			}
			account := hub.Store.GetOrCreatePlayer(p.IP, p.Fingerprint, name)
			if !hub.JoinIfAllowed(p, account, name, primary, secondary, skin, primaryWeaponSkin, secondaryWeaponSkin) {
				p.Send(Reject("访问已被封禁"))
				time.Sleep(20 * time.Millisecond)
				return
			}
		case OpInput:
			if !p.joined || len(payload) < 11 {
				continue
			}
			seq := binary.LittleEndian.Uint16(payload)
			keys := payload[2]
			yaw := float64(math.Float32frombits(binary.LittleEndian.Uint32(payload[3:])))
			pitch := float64(math.Float32frombits(binary.LittleEndian.Uint32(payload[7:])))
			if !finite(yaw) || !finite(pitch) {
				continue
			}
			pitch = math.Max(-1.55, math.Min(1.55, pitch))
			yaw = math.Remainder(yaw, 2*math.Pi)
			p.queueInput(seq, keys, yaw, pitch, now)
		case OpFire:
			if !p.joined || len(payload) < 15 {
				continue
			}
			shot := binary.LittleEndian.Uint16(payload)
			seen := binary.LittleEndian.Uint32(payload[2:])
			mode := payload[6]
			yaw := float64(math.Float32frombits(binary.LittleEndian.Uint32(payload[7:])))
			pitch := float64(math.Float32frombits(binary.LittleEndian.Uint32(payload[11:])))
			if room := p.Room; room != nil {
				room.mu.Lock()
				room.TryFire(&p.PlayerState, yaw, pitch, mode, seen, shot, now)
				room.mu.Unlock()
			}
		case OpReload:
			if room := p.Room; room != nil {
				room.mu.Lock()
				room.StartReload(&p.PlayerState, now)
				room.mu.Unlock()
			}
		case OpGrenade:
			if len(payload) < 8 {
				continue
			}
			yaw := float64(math.Float32frombits(binary.LittleEndian.Uint32(payload)))
			pitch := float64(math.Float32frombits(binary.LittleEndian.Uint32(payload[4:])))
			if room := p.Room; room != nil && finite(yaw) && finite(pitch) {
				yaw = math.Remainder(yaw, 2*math.Pi)
				pitch = math.Max(-1.55, math.Min(1.55, pitch))
				room.mu.Lock()
				room.ThrowGrenade(&p.PlayerState, yaw, pitch, now)
				room.mu.Unlock()
			}
		case OpSwitch:
			if len(payload) < 1 {
				continue
			}
			if room := p.Room; room != nil {
				room.mu.Lock()
				if p.Alive {
					p.SwitchSlot(payload[0])
				}
				room.mu.Unlock()
			}
		case OpLoadout:
			if len(payload) < 4 || !validLoadout(payload[0], payload[1]) {
				continue
			}
			if room := p.Room; room != nil {
				room.mu.Lock()
				if !p.Alive {
					p.Primary, p.Secondary = payload[0], payload[1]
					p.PrimaryWeaponSkin = hub.Store.UnlockedWeaponSkin(p.Account, payload[0], payload[2])
					p.SecondaryWeaponSkin = hub.Store.UnlockedWeaponSkin(p.Account, payload[1], payload[3])
				}
				room.mu.Unlock()
			}
		case OpRosterRequest:
			if room := p.Room; room != nil {
				room.mu.Lock()
				p.rosterRequested = true
				room.mu.Unlock()
			}
		case OpToggleFlight:
			if room := p.Room; room != nil {
				room.mu.Lock()
				if p.Alive {
					p.Flying = !p.Flying
					p.Crouch = false
					p.Vel.Y = 0
					room.Emit(Event{Type: EvFlightToggle, Player: p.Id, Kind: boolByte(p.Flying), Name: p.Name})
				}
				room.mu.Unlock()
			}
		case OpUltimate:
			if len(payload) < 1 {
				continue
			}
			if room := p.Room; room != nil {
				room.mu.Lock()
				room.CastUltimate(&p.PlayerState, payload[0], now)
				room.mu.Unlock()
			}
		case OpHex:
			if len(payload) < 1 {
				continue
			}
			if room := p.Room; room != nil {
				room.mu.Lock()
				room.HexPick(&p.PlayerState, payload[0], now)
				room.mu.Unlock()
			}
		case OpChat:
			if !p.joined || len(payload) < 1 {
				continue
			}
			text := sanitizeChat(string(payload))
			if text == "" || now.Before(p.NextChatAt) {
				continue
			}
			p.NextChatAt = now.Add(chatCooldown)
			if room := p.Room; room != nil {
				room.mu.Lock()
				room.Emit(Event{Type: EvChat, Player: p.Id, Name: p.Name, Message: text})
				room.mu.Unlock()
			}
		case OpPing:
			out := make([]byte, 5)
			out[0] = OpPong
			binary.LittleEndian.PutUint32(out[1:], uint32(max(0, min(outboundBPS.Load(), int64(^uint32(0))))))
			p.Send(out)
		}
	}
}

func boolByte(value bool) uint8 {
	if value {
		return 1
	}
	return 0
}

func (p *Player) queueInput(seq uint16, keys uint8, yaw, pitch float64, at time.Time) {
	p.inputMu.Lock()
	p.queuedInput = playerInput{seq: seq, keys: keys, yaw: yaw, pitch: pitch, at: at}
	p.hasQueuedInput = true
	p.inputMu.Unlock()
}

func (p *Player) applyQueuedInput() {
	p.inputMu.Lock()
	if !p.hasQueuedInput {
		p.inputMu.Unlock()
		return
	}
	input := p.queuedInput
	p.hasQueuedInput = false
	p.inputMu.Unlock()
	crouchPressed := input.keys&KeyCrouch != 0 && p.CmdKeys&KeyCrouch == 0
	if input.keys&KeyAim != 0 && (p.CmdKeys&KeyAim == 0 || p.AimStarted.IsZero()) {
		p.AimStarted = input.at
	} else if input.keys&KeyAim == 0 {
		p.AimStarted = time.Time{}
	}
	p.CmdKeys, p.Yaw, p.Pitch, p.LastInputSeq = input.keys, input.yaw, input.pitch, input.seq
	if crouchPressed {
		if room := p.Room; room != nil {
			room.NoteCrouchTap(&p.PlayerState, input.at)
		}
	}
}

func (p *Player) Send(msg []byte) {
	isSnapshot := len(msg) > 0 && msg[0] == OpSnapshot
	if p.IsBot {
		if isSnapshot {
			p.releaseSnapshot(msg)
		}
		return
	}
	p.sendMu.RLock()
	if p.closed || p.send == nil {
		p.sendMu.RUnlock()
		if isSnapshot {
			p.releaseSnapshot(msg)
		}
		return
	}
	if isSnapshot {
		select {
		case p.latestSnapshot <- msg:
		default:
			select {
			case stale := <-p.latestSnapshot:
				p.releaseSnapshot(stale)
				p.releaseSnapshot(msg)
				p.netReset.Store(true)
				p.sendMu.RUnlock()
				return
			default:
			}
			select {
			case p.latestSnapshot <- msg:
			default:
				p.releaseSnapshot(msg)
				p.netReset.Store(true)
			}
		}
		p.sendMu.RUnlock()
		return
	}
	select {
	case p.send <- msg:
		p.sendMu.RUnlock()
	default:
		p.sendMu.RUnlock()
		droppedMessages.Add(1)
		p.closeOnce.Do(func() { go p.ws.Close() })
	}
}
func (p *Player) writePump() {
	defer p.ws.Close()
	ping := time.NewTicker(pingPeriod)
	defer ping.Stop()
	writes := 0
	for {
		var msg []byte
		var ok, isSnapshot bool
		select {
		case msg, ok = <-p.send:
		default:
			select {
			case msg, ok = <-p.send:
			case msg = <-p.latestSnapshot:
				ok, isSnapshot = true, true
			case <-ping.C:
				deadline := time.Now().Add(5 * time.Second)
				p.ws.SetWriteDeadline(deadline)
				if err := p.ws.WriteControl(websocket.PingMessage, nil, deadline); err != nil {
					return
				}
				continue
			}
		}
		if !ok {
			select {
			case msg = <-p.latestSnapshot:
				p.releaseSnapshot(msg)
			default:
			}
			return
		}
		if writes%writeDeadlineEvery == 0 {
			p.ws.SetWriteDeadline(time.Now().Add(5 * time.Second))
		}
		writes++
		err := p.ws.WriteMessage(websocket.BinaryMessage, msg)
		if isSnapshot {
			p.releaseSnapshot(msg)
		}
		if err != nil {
			return
		}
		outboundBytes.Add(int64(len(msg)))
	}
}

func sanitizeName(s string) string {
	s = strings.ToValidUTF8(s, "")
	r := []rune(strings.TrimSpace(s))
	if len(r) > 16 {
		r = r[:16]
	}
	out := r[:0]
	for _, c := range r {
		if c != '\n' && c != '\r' && c != '\t' && c != 0 && c != '<' && c != '>' && c != '"' && c != '\'' {
			out = append(out, c)
		}
	}
	return string(out)
}

func sanitizeChat(s string) string {
	s = strings.ToValidUTF8(s, "")
	s = strings.TrimSpace(s)
	r := []rune(s)
	if len(r) > maxChatRunes {
		r = r[:maxChatRunes]
	}
	out := r[:0]
	for _, c := range r {
		if c == 0 || c == '\n' || c == '\r' || c == '\t' || (c < 0x20 && c != ' ') {
			continue
		}
		if utf8.RuneLen(c) < 0 {
			continue
		}
		out = append(out, c)
	}
	return string(out)
}
func sanitizeFingerprint(s string) string {
	if len(s) > 96 {
		s = s[:96]
	}
	var b strings.Builder
	for _, c := range s {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-' {
			b.WriteRune(c)
		}
	}
	return b.String()
}
