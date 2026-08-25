package main

import (
	"log"
	"sync"
	"sync/atomic"
	"time"
)

var OnlinePlayers atomic.Int64

type Hub struct {
	World    *World
	Store    *Store
	mu       sync.Mutex
	banMu    sync.RWMutex
	rooms    []*Room
	nextId   int
	botCount int
}

type AdminPlayer struct {
	Id     uint16 `json:"id"`
	Name   string `json:"name"`
	IP     string `json:"ip"`
	Room   int    `json:"room"`
	Alive  bool   `json:"alive"`
	HP     uint8  `json:"hp"`
	Kills  uint16 `json:"kills"`
	Deaths uint16 `json:"deaths"`
}

func NewHub(w *World, s *Store) *Hub { return &Hub{World: w, Store: s, botCount: 8} }

func (h *Hub) Broadcast(msg []byte) {
	h.mu.Lock()
	players := make([]*Player, 0, OnlinePlayers.Load())
	for _, room := range h.rooms {
		room.mu.Lock()
		for _, p := range room.Players {
			if !p.IsBot && p.ready {
				players = append(players, p)
			}
		}
		room.mu.Unlock()
	}
	h.mu.Unlock()
	for _, p := range players {
		p.Send(msg)
	}
}

func (h *Hub) BotStatus() (count, rooms int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.botCount, len(h.rooms)
}

func (h *Hub) OnlineSnapshot() []AdminPlayer {
	h.mu.Lock()
	defer h.mu.Unlock()
	players := make([]AdminPlayer, 0)
	for _, room := range h.rooms {
		room.mu.Lock()
		for _, p := range room.Players {
			if p.IsBot {
				continue
			}
			players = append(players, AdminPlayer{
				Id: p.Id, Name: p.Name, IP: p.IP, Room: room.Id, Alive: p.Alive,
				HP: p.HP, Kills: p.Kills, Deaths: p.Deaths,
			})
		}
		room.mu.Unlock()
	}
	return players
}

func (h *Hub) JoinIfAllowed(p *Player, account, name string, primary, secondary, skin, primaryWeaponSkin, secondaryWeaponSkin uint8) bool {
	h.banMu.RLock()
	defer h.banMu.RUnlock()
	if h.Store.IsIPBanned(p.IP) {
		return false
	}
	h.Join(p, account, name, primary, secondary, skin, primaryWeaponSkin, secondaryWeaponSkin)
	return true
}

func (h *Hub) BanIP(ip, reason string) (int, error) {
	h.banMu.Lock()
	defer h.banMu.Unlock()
	if err := h.Store.AddIPBan(ip, reason); err != nil {
		return 0, err
	}
	return h.KickIP(ip), nil
}

func (h *Hub) KickIP(ip string) int {
	ip, err := normalizeBanIP(ip)
	if err != nil {
		return 0
	}
	h.mu.Lock()
	var kicked []*Player
	for _, room := range h.rooms {
		room.mu.Lock()
		for _, p := range room.Players {
			if !p.IsBot && p.IP == ip {
				kicked = append(kicked, p)
			}
		}
		room.mu.Unlock()
	}
	h.mu.Unlock()

	for _, p := range kicked {
		if p.ws != nil {
			p.closeOnce.Do(func() { _ = p.ws.Close() })
		}
	}
	return len(kicked)
}

func (h *Hub) SetBotCount(count int) (int, int) {
	count = max(0, min(count, len(BotNames)))
	h.mu.Lock()
	defer h.mu.Unlock()
	h.botCount = count
	active := 0
	for _, room := range h.rooms {
		room.mu.Lock()
		if !room.closed {
			room.SetBotCount(count)
			active++
		}
		room.mu.Unlock()
	}
	return count, active
}

func (h *Hub) Join(p *Player, account, name string, primary, secondary, skin, primaryWeaponSkin, secondaryWeaponSkin uint8) {
	h.mu.Lock()
	var room *Room
	for _, candidate := range h.rooms {
		candidate.mu.Lock()
		if !candidate.closed && candidate.HumanCountLocked() < RoomCap {
			room = candidate
			break
		}
		candidate.mu.Unlock()
	}
	if room == nil {
		h.nextId++
		room = NewRoom(h.nextId, h.World, h.Store)
		room.SetBotCount(h.botCount)
		h.rooms = append(h.rooms, room)
		room.mu.Lock()
	}
	// Bots are filler: a real player always owns the seat.
	if len(room.Players) >= RoomCap {
		for i := len(room.Players) - 1; i >= 0; i-- {
			if bot := room.Players[i]; bot.IsBot {
				delete(room.botAIs, bot.Id)
				delete(room.history, bot.Id)
				for _, other := range room.Players {
					delete(other.netCache, bot.Id)
					delete(other.netFullAt, bot.Id)
				}
				room.Emit(Event{Type: EvPlayerLeave, Player: bot.Id})
				room.Players = append(room.Players[:i], room.Players[i+1:]...)
				break
			}
		}
	}
	p.Id = room.allocPlayerID()
	p.Account = account
	p.Name = name
	p.PrimaryWeaponSkin = h.Store.UnlockedWeaponSkin(account, primary, primaryWeaponSkin)
	p.SecondaryWeaponSkin = h.Store.UnlockedWeaponSkin(account, secondary, secondaryWeaponSkin)
	p.joined = true
	p.Room = room
	p.ApplyLoadout(primary, secondary)
	p.Skin = skin
	p.HP = MaxHP
	p.Armor = 100
	p.Alive = true
	p.OnGround = true
	p.InvincibleUntil = time.Now().Add(SpawnProtectS)
	p.Pos = room.BestSpawn(&p.PlayerState)
	room.Players = append(room.Players, p)
	room.Emit(Event{Type: EvPlayerName, Player: p.Id, Name: p.Name})
	p.Send(Welcome(p.Id, h.World.Revision))
	p.Send(Roster(room.Players))
	p.lastSelf, p.hasLastSelf = compactSelf(&p.PlayerState), true
	p.Send(SelfState(&p.PlayerState))
	if pickups := room.pickupEvents(); len(pickups) > 0 {
		p.Send(Events(pickups))
	}
	p.ready = true
	if !room.running {
		room.running = true
		go room.Run()
	}
	room.mu.Unlock()
	h.mu.Unlock()

	OnlinePlayers.Add(1)
	h.Store.IncrMeta("total_joins", 1)
	log.Printf("player %d joined room %d as %q", p.Id, room.Id, name)
}

func (h *Hub) Leave(p *Player) {
	p.sendMu.Lock()
	if p.closed {
		p.sendMu.Unlock()
		return
	}
	p.closed = true
	if p.send != nil {
		close(p.send)
	}
	p.sendMu.Unlock()
	if room := p.Room; room != nil && p.joined {
		room.Remove(p)
		OnlinePlayers.Add(-1)
		log.Printf("player %d (%s) left room %d", p.Id, p.Name, room.Id)
	}
	if p.joined {
		h.Store.Flush()
		h.Store.Invalidate(p.IP, p.Fingerprint)
	}
	h.mu.Lock()
	alive := h.rooms[:0]
	for _, r := range h.rooms {
		if !r.Empty() {
			alive = append(alive, r)
		}
	}
	h.rooms = alive
	h.mu.Unlock()
}
