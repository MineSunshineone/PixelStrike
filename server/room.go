package main

import (
	"log"
	"sync"
	"time"
)

type Room struct {
	Id                    int
	World                 *World
	Store                 *Store
	mu                    sync.Mutex
	running, closed       bool
	Players               []*Player
	Grenades              []*Grenade
	Pickups               []Pickup
	Chickens              []Chicken
	nextNadeId, nextIdSeq uint16
	nextChickenId         uint16
	nextChickenRainAt     time.Time
	tick                  uint32
	pending               []Event
	botAIs                map[uint16]*BotAI
	history               map[uint16]*poseHistory
	teamAttempts          map[uint16]*teamAttempt
	outboundBuf           []outbound
	quantizedBuf          []quantState
}

// teamAttempt tracks the consecutive crouch taps of one human for illegal teaming.
type teamAttempt struct {
	count   int
	firstAt time.Time
	lastAt  time.Time
}

const RoomCap = 100

func NewRoom(id int, w *World, s *Store) *Room {
	r := &Room{Id: id, World: w, Store: s, nextIdSeq: 1, botAIs: make(map[uint16]*BotAI), history: make(map[uint16]*poseHistory), teamAttempts: make(map[uint16]*teamAttempt)}
	r.initPickups()
	r.initChickens()
	return r
}

func (r *Room) Remove(p *Player) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Clear bond: if this player had a bond mate, break the bond
	if p.BondMate != 0 {
		if mate := r.findPlayer(p.BondMate); mate != nil {
			mate.BondMate = 0
		}
	}
	// 离开即解散非法小队，避免残留悬空的队友引用。
	r.BreakIllegalTeam(&p.PlayerState)
	for i, q := range r.Players {
		if q == p {
			r.Players = append(r.Players[:i], r.Players[i+1:]...)
			break
		}
	}
	delete(r.botAIs, p.Id)
	delete(r.history, p.Id)
	for _, other := range r.Players {
		delete(other.netCache, p.Id)
		delete(other.netFullAt, p.Id)
	}
	if !p.IsBot && r.HumanCountLocked() == 0 {
		r.closed, r.Players = true, nil
		r.botAIs = make(map[uint16]*BotAI)
		r.history = make(map[uint16]*poseHistory)
		r.Grenades, r.Pickups, r.pending = nil, nil, nil
		r.Chickens = nil
		return
	}
	r.Emit(Event{Type: EvPlayerLeave, Player: p.Id})
}
func (r *Room) Empty() bool { r.mu.Lock(); defer r.mu.Unlock(); return r.closed || len(r.Players) == 0 }
func (r *Room) HumanCountLocked() int {
	n := 0
	for _, p := range r.Players {
		if !p.IsBot {
			n++
		}
	}
	return n
}

func (r *Room) allocPlayerID() uint16 {
	for {
		id := r.nextIdSeq
		r.nextIdSeq++
		if id != 0 && r.findPlayer(id) == nil {
			return id
		}
	}
}

type outbound struct {
	p                              *Player
	snapshot, self, events, roster []byte
}

func (r *Room) Run() {
	ticker := time.NewTicker(time.Second / TickRate)
	defer ticker.Stop()
	for now := range ticker.C {
		start := time.Now()
		r.mu.Lock()
		if r.closed || len(r.Players) == 0 {
			r.running = false
			r.mu.Unlock()
			return
		}
		for _, p := range r.Players {
			p.applyQueuedInput()
		}
		r.FinishReloads(now)
		r.Step(now)
		evts := r.pending
		r.pending = nil
		needSnap := r.tick%2 == 0
		var outs []outbound
		if needSnap || len(evts) > 0 {
			players := r.Players
			outs = r.outboundBuf[:0]
			if needSnap {
				r.quantizedBuf = quantizePlayers(r.quantizedBuf, players, now.UnixNano())
			}
			var periodicRoster []byte
			if needSnap && r.tick%600 == 0 {
				periodicRoster = Roster(players)
			}
			for _, p := range players {
				if p.IsBot || !p.ready {
					continue
				}
				out := outbound{p: p}
				if needSnap {
					out.snapshot = p.BuildSnapshot(r.tick, players, r.quantizedBuf, now)
					self := compactSelf(&p.PlayerState)
					if r.tick%60 == 0 || !p.hasLastSelf || self != p.lastSelf {
						out.self = SelfState(&p.PlayerState)
						p.lastSelf, p.hasLastSelf = self, true
					}
					if periodicRoster != nil {
						out.roster = periodicRoster
					} else if p.rosterRequested {
						out.roster = Roster(players)
					}
					if periodicRoster != nil || p.rosterRequested {
						p.rosterRequested = false
					}
				}
				if len(evts) > 0 {
					if filtered := r.eventsFor(p, evts); len(filtered) > 0 {
						out.events = Events(filtered)
					}
				}
				if out.snapshot != nil || out.self != nil || out.events != nil || out.roster != nil {
					outs = append(outs, out)
				}
			}
		}
		r.tick++
		r.mu.Unlock()
		for i := range outs {
			out := &outs[i]
			if out.snapshot != nil {
				out.p.Send(out.snapshot)
			}
			if out.self != nil {
				out.p.Send(out.self)
			}
			if out.events != nil {
				out.p.Send(out.events)
			}
			if out.roster != nil {
				out.p.Send(out.roster)
			}
			*out = outbound{}
		}
		if outs != nil {
			r.outboundBuf = outs[:0]
		}
		took := time.Since(start)
		recordTick(took)
		if took > time.Second/TickRate {
			log.Printf("room %d: slow tick %v", r.Id, took)
		}
	}
}

func (r *Room) eventsFor(target *Player, evts []Event) []Event {
	out := make([]Event, 0, len(evts))
	for _, e := range evts {
		send := false
		switch e.Type {
		case EvKill, EvPlayerName, EvPlayerLeave, EvFlightToggle, EvRevenge, EvBondEvent, EvChat:
			send = true
		case EvUltimate:
			send = true
		case EvStreakBuff:
			send = e.Player == target.Id
		case EvHit:
			send = e.Player == target.Id || e.Victim == target.Id
		case EvRespawn:
			send = e.Player == target.Id || horizontalWithin(target.Pos, e.Origin, 120)
		case EvExplosion, EvNadeThrow:
			send = horizontalWithin(target.Pos, e.Origin, 120)
		case EvPickupSpawn, EvPickupTaken:
			send = true
		case EvChickenSpawn:
			send = horizontalWithin(target.Pos, e.Origin, 120)
		case EvChickenDeath:
			send = true
		case EvReloadStart:
			if e.Player == target.Id {
				send = true
			} else if p := r.findPlayer(e.Player); p != nil {
				send = horizontalWithin(target.Pos, p.Pos, 80)
			}
		}
		if send {
			out = append(out, e)
		}
	}
	return out
}
func horizontalWithin(a, b Vec3, distance float64) bool {
	dx, dz := a.X-b.X, a.Z-b.Z
	return dx*dx+dz*dz <= distance*distance
}
func (r *Room) findPlayer(id uint16) *Player {
	for _, p := range r.Players {
		if p.Id == id {
			return p
		}
	}
	return nil
}
func (r *Room) Emit(e Event) { r.pending = append(r.pending, e) }
