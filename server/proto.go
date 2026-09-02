package main

import (
	"encoding/binary"
	"math"
	"unicode/utf8"
)

const ProtocolVersion = 10
const SkinCount uint8 = 8

const (
	OpJoin          = 0x01
	OpInput         = 0x02
	OpFire          = 0x03
	OpReload        = 0x04
	OpGrenade       = 0x06
	OpSwitch        = 0x08
	OpLoadout       = 0x09
	OpRosterRequest = 0x0A
	OpToggleFlight  = 0x0B
	OpUltimate      = 0x0C
	OpChat          = 0x0D

	OpWelcome     = 0x81
	OpSnapshot    = 0x82
	OpEvents      = 0x83
	OpPong        = 0x84
	OpSelf        = 0x86
	OpRoster      = 0x87
	OpReject      = 0x88
	OpMaintenance = 0x89
	OpPing        = 0xF0
)

const (
	EvKill = iota
	EvHit
	EvRespawn
	_
	_
	EvReloadStart
	EvPlayerName
	EvExplosion
	EvNadeThrow
	EvPlayerLeave
	EvPickupSpawn
	EvPickupTaken
	EvFlightToggle
	EvStreakBuff
	EvRevenge
	EvBondEvent
	EvUltimate
	EvChat
	EvChickenSpawn
	EvChickenDeath
	EvZombieSpawn
	EvZombieDeath
)

type Event struct {
	Type                        uint8
	Killer, Victim, Player      uint16
	Headshot, Weapon, Dmg, Kind uint8
	Origin, Dir                 Vec3
	Ms                          uint16
	Name                        string
	Message                     string
}

type Buf struct{ b []byte }

func NewBuf(op byte) *Buf   { return &Buf{b: []byte{op}} }
func (w *Buf) U8(v uint8)   { w.b = append(w.b, v) }
func (w *Buf) I8(v int8)    { w.b = append(w.b, byte(v)) }
func (w *Buf) U16(v uint16) { w.b = binary.LittleEndian.AppendUint16(w.b, v) }
func (w *Buf) I16(v int16)  { w.b = binary.LittleEndian.AppendUint16(w.b, uint16(v)) }
func (w *Buf) U32(v uint32) { w.b = binary.LittleEndian.AppendUint32(w.b, v) }
func (w *Buf) F32(v float64) {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		v = 0
	}
	w.b = binary.LittleEndian.AppendUint32(w.b, math.Float32bits(float32(v)))
}
func (w *Buf) V3(v Vec3)     { w.F32(v.X); w.F32(v.Y); w.F32(v.Z) }
func (w *Buf) Bytes() []byte { return w.b }

func Welcome(id uint16, mapRevision uint32) []byte {
	w := NewBuf(OpWelcome)
	w.U8(ProtocolVersion)
	w.U16(id)
	w.U32(mapRevision)
	return w.Bytes()
}

func Reject(reason string) []byte {
	w := NewBuf(OpReject)
	b := []byte(reason)
	if len(b) > 200 {
		b = b[:200]
	}
	w.U8(uint8(len(b)))
	w.b = append(w.b, b...)
	return w.Bytes()
}

func Maintenance(retryAfter uint8) []byte {
	return []byte{OpMaintenance, retryAfter}
}

type compactSelfState struct {
	slot, weapon, weaponSkin, mag, nades, ultimatePoints, ultimate uint8
	reserve                                                        uint16
}

func compactSelf(p *PlayerState) compactSelfState {
	mag, reserve := p.ActiveAmmo()
	return compactSelfState{
		slot: p.ActiveSlot, weapon: p.Weapon, weaponSkin: p.WeaponSkin,
		mag: uint8(max(0, min(mag, 255))), reserve: uint16(max(0, min(reserve, 65535))),
		nades: uint8(max(0, min(p.Grenades, 255))), ultimatePoints: p.UltimatePoints, ultimate: p.Ultimate,
	}
}

func SelfState(p *PlayerState) []byte {
	state := compactSelf(p)
	w := NewBuf(OpSelf)
	w.U16(p.LastInputSeq)
	w.U8(state.slot)
	w.U8(state.weapon)
	w.U8(state.weaponSkin)
	w.U8(state.mag)
	w.U16(state.reserve)
	w.U8(state.nades)
	w.U8(state.ultimatePoints)
	w.U8(state.ultimate)
	return w.Bytes()
}

func Roster(players []*Player) []byte {
	w := NewBuf(OpRoster)
	w.U8(uint8(min(len(players), 255)))
	for _, p := range players {
		w.U16(p.Id)
		w.U16(p.Kills)
		w.U16(p.Deaths)
		nb := safeNameBytes(p.Name)
		w.U8(uint8(len(nb)))
		w.b = append(w.b, nb...)
	}
	return w.Bytes()
}

func Events(evts []Event) []byte {
	w := NewBuf(OpEvents)
	w.U8(uint8(min(len(evts), 255)))
	for _, e := range evts {
		w.U8(e.Type)
		switch e.Type {
		case EvKill:
			w.U16(e.Killer)
			w.U16(e.Victim)
			w.U8(e.Weapon)
			w.U8(e.Headshot)
		case EvHit:
			w.U16(e.Player)
			w.U16(e.Victim)
			w.U8(e.Dmg | e.Headshot<<7)
		case EvRespawn:
			w.U16(e.Player)
			w.V3(e.Origin)
		case EvPlayerLeave:
			w.U16(e.Player)
		case EvReloadStart:
			w.U16(e.Player)
			w.U16(e.Ms)
		case EvPlayerName:
			w.U16(e.Player)
			nb := safeNameBytes(e.Name)
			w.U8(uint8(len(nb)))
			w.b = append(w.b, nb...)
		case EvExplosion:
			w.V3(e.Origin)
		case EvNadeThrow:
			w.U16(e.Player)
			w.V3(e.Origin)
			w.V3(e.Dir)
		case EvPickupSpawn:
			w.U16(e.Player)
			w.U8(e.Kind)
			w.V3(e.Origin)
		case EvPickupTaken:
			w.U16(e.Player)
			w.U16(e.Victim)
			w.U8(e.Kind)
			w.U16(e.Ms)
		case EvFlightToggle:
			w.U16(e.Player)
			w.U8(e.Kind)
			nb := safeNameBytes(e.Name)
			w.U8(uint8(len(nb)))
			w.b = append(w.b, nb...)
		case EvStreakBuff:
			w.U16(e.Player)
			w.U8(e.Kind)
			w.U8(e.Dmg)
			w.U16(e.Ms)
		case EvRevenge:
			w.U16(e.Player)
			nb := safeNameBytes(e.Name)
			w.U8(uint8(len(nb)))
			w.b = append(w.b, nb...)
		case EvBondEvent:
			w.U16(e.Player)
			w.U16(e.Victim)
			w.U8(e.Kind)
			w.U8(e.Dmg)
			nb := safeNameBytes(e.Name)
			w.U8(uint8(len(nb)))
			w.b = append(w.b, nb...)
		case EvUltimate:
			w.U16(e.Player)
			w.U8(e.Kind)
			w.U16(e.Ms)
			nb := safeNameBytes(e.Name)
			w.U8(uint8(len(nb)))
			w.b = append(w.b, nb...)
		case EvChat:
			w.U16(e.Player)
			nb := safeNameBytes(e.Name)
			w.U8(uint8(len(nb)))
			w.b = append(w.b, nb...)
			mb := safeChatBytes(e.Message)
			w.U8(uint8(len(mb)))
			w.b = append(w.b, mb...)
		case EvChickenSpawn:
			w.U16(e.Player)
			w.V3(e.Origin)
			w.V3(e.Dir)
		case EvChickenDeath:
			w.U16(e.Killer)
			w.U16(e.Victim)
			w.V3(e.Origin)
			w.U8(e.Weapon)
		case EvZombieSpawn:
			w.U16(e.Player)
			w.V3(e.Origin)
			w.V3(e.Dir)
		case EvZombieDeath:
			w.U16(e.Killer)
			w.U16(e.Victim)
			w.V3(e.Origin)
			w.U8(e.Weapon)
		}
	}
	return w.Bytes()
}

func safeNameBytes(name string) []byte {
	b := []byte(name)
	for len(b) > 64 || !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return b
}

func safeChatBytes(text string) []byte {
	b := []byte(text)
	for len(b) > maxChatBytes || !utf8.Valid(b) {
		b = b[:len(b)-1]
	}
	return b
}
