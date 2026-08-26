package main

import (
	"database/sql"
	"errors"
	"log"
	"math"
	"math/rand/v2"
	"net"
	"net/netip"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

type Store struct {
	db      *sql.DB
	ch      chan delta
	flushCh chan chan struct{}
	stopCh  chan chan struct{}
	cacheMu sync.RWMutex
	cache   map[string]string
	banMu   sync.RWMutex
	ipBans  map[string]IPBan
}

type IPBan struct {
	IP        string `json:"ip"`
	Reason    string `json:"reason"`
	CreatedAt int64  `json:"createdAt"`
}

type delta struct {
	name        string
	kills       int
	deaths      int
	weapon      int
	weaponKills int
}

const (
	GoldKillRequirement    = 100
	DiamondKillRequirement = 500
)

type WeaponProgress struct {
	Weapon  uint8 `json:"weapon"`
	Kills   int   `json:"kills"`
	Gold    bool  `json:"gold"`
	Diamond bool  `json:"diamond"`
}

func NewStore(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)&_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	for _, schema := range []string{
		`CREATE TABLE IF NOT EXISTS stats(
			name TEXT PRIMARY KEY,
			ip TEXT DEFAULT '',
			fingerprint TEXT DEFAULT '',
			kills INTEGER DEFAULT 0,
			deaths INTEGER DEFAULT 0,
			money INTEGER DEFAULT 0,
			updated_at INTEGER)`,
		`CREATE TABLE IF NOT EXISTS meta(
			key TEXT PRIMARY KEY, val INTEGER DEFAULT 0)`,
		`CREATE TABLE IF NOT EXISTS ip_bans(
			ip TEXT PRIMARY KEY,
			reason TEXT NOT NULL DEFAULT '',
			created_at INTEGER NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS weapon_kills(
			account TEXT NOT NULL,
			weapon INTEGER NOT NULL,
			kills INTEGER NOT NULL DEFAULT 0,
			updated_at INTEGER NOT NULL,
			PRIMARY KEY(account, weapon))`,
	} {
		if _, err := db.Exec(schema); err != nil {
			return nil, err
		}
	}
	if _, err := db.Exec(`INSERT OR IGNORE INTO meta(key,val) VALUES('bot_count',8)`); err != nil {
		return nil, err
	}
	// Migrate pre-binding databases: CREATE TABLE IF NOT EXISTS won't add
	// columns to an existing stats table.
	for _, col := range []string{"ip", "fingerprint"} {
		var have int
		if err := db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('stats') WHERE name=?`, col).Scan(&have); err == nil && have == 0 {
			if _, err := db.Exec("ALTER TABLE stats ADD COLUMN " + col + " TEXT DEFAULT ''"); err != nil {
				return nil, err
			}
		}
	}
	for _, index := range []string{
		`CREATE INDEX IF NOT EXISTS stats_fingerprint_idx ON stats(fingerprint) WHERE fingerprint != ''`,
		`CREATE INDEX IF NOT EXISTS stats_ip_idx ON stats(ip) WHERE ip != ''`,
	} {
		if _, err := db.Exec(index); err != nil {
			return nil, err
		}
	}
	// Clean up historical bot records from persistent stats ([BOT] in-game
	// bots plus legacy tools/bots.mjs load-test names).
	_, _ = db.Exec(`DELETE FROM stats WHERE name LIKE '[BOT]%' OR name LIKE 'bot%' OR name LIKE 'CombatBot%'
		OR name LIKE 'Duel[AB]%' OR name LIKE 'load-%'
		OR name IN ('VoxelKing','ShadowSniper','ApexGhost','ViperZero','Phoenix','Maverick','CyberWolf',
			'Soldier','VoxelMaster','BugHunter','ColTest','Commander','Tester','ApexHunter','General')`)

	ipBans := make(map[string]IPBan)
	rows, err := db.Query(`SELECT ip, reason, created_at FROM ip_bans`)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	for rows.Next() {
		var ban IPBan
		if err := rows.Scan(&ban.IP, &ban.Reason, &ban.CreatedAt); err != nil {
			_ = rows.Close()
			_ = db.Close()
			return nil, err
		}
		ip, err := normalizeBanIP(ban.IP)
		if err != nil {
			continue
		}
		ban.IP = ip
		ipBans[ip] = ban
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		_ = db.Close()
		return nil, err
	}
	_ = rows.Close()

	s := &Store{
		db:      db,
		ch:      make(chan delta, 4096),
		flushCh: make(chan chan struct{}, 16),
		stopCh:  make(chan chan struct{}, 1),
		cache:   make(map[string]string),
		ipBans:  ipBans,
	}
	go s.writer()
	return s, nil
}

const upsert = `INSERT INTO stats(name,kills,deaths,updated_at) VALUES(?,?,?,strftime('%s','now'))
	ON CONFLICT(name) DO UPDATE SET kills=kills+excluded.kills,
	deaths=deaths+excluded.deaths, updated_at=excluded.updated_at`

func (s *Store) writer() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	batch := make(map[string]delta)
	flush := func() {
		if len(batch) == 0 {
			return
		}
		tx, err := s.db.Begin()
		if err != nil {
			log.Printf("store begin: %v", err)
			return
		}
		for _, d := range batch {
			if _, err := tx.Exec(upsert, d.name, d.kills, d.deaths); err != nil {
				_ = tx.Rollback()
				log.Printf("store upsert: %v", err)
				return
			}
			if d.weapon >= 0 && d.weaponKills > 0 {
				if _, err := tx.Exec(`INSERT INTO weapon_kills(account,weapon,kills,updated_at) VALUES(?,?,?,strftime('%s','now'))
					ON CONFLICT(account,weapon) DO UPDATE SET kills=kills+excluded.kills, updated_at=excluded.updated_at`, d.name, d.weapon, d.weaponKills); err != nil {
					_ = tx.Rollback()
					log.Printf("store weapon upsert: %v", err)
					return
				}
			}
		}
		if err := tx.Commit(); err != nil {
			log.Printf("store commit: %v", err)
			return
		}
		clear(batch)
	}
	for {
		select {
		case <-ticker.C:
			flush()
		case d := <-s.ch:
			key := d.name + "\x00" + strconv.Itoa(d.weapon)
			acc := batch[key]
			acc.name = d.name
			acc.kills += d.kills
			acc.deaths += d.deaths
			acc.weaponKills += d.weaponKills
			acc.weapon = d.weapon
			batch[key] = acc
		case done := <-s.flushCh:
			// Drain all pending deltas in channel before flushing
			for {
				select {
				case d := <-s.ch:
					key := d.name + "\x00" + strconv.Itoa(d.weapon)
					acc := batch[key]
					acc.name = d.name
					acc.kills += d.kills
					acc.deaths += d.deaths
					acc.weaponKills += d.weaponKills
					acc.weapon = d.weapon
					batch[key] = acc
				default:
					goto drained
				}
			}
		drained:
			flush()
			if done != nil {
				close(done)
			}
		case done := <-s.stopCh:
			for {
				select {
				case d := <-s.ch:
					key := d.name + "\x00" + strconv.Itoa(d.weapon)
					acc := batch[key]
					acc.name = d.name
					acc.kills += d.kills
					acc.deaths += d.deaths
					acc.weaponKills += d.weaponKills
					acc.weapon = d.weapon
					batch[key] = acc
				default:
					flush()
					close(done)
					return
				}
			}
		}
	}
}

// Accumulate queues a stat change; persisted on the next 30s tick.
func (s *Store) Accumulate(name string, kills, deaths int) {
	select {
	case s.ch <- delta{name: name, kills: kills, deaths: deaths, weapon: -1}:
	default:
		log.Printf("store queue full, dropping stats for %q", name)
	}
}

func (s *Store) AccumulateWeaponKill(name string, weapon uint8) {
	select {
	case s.ch <- delta{name: name, weapon: int(weapon), weaponKills: 1}:
	default:
		log.Printf("store queue full, dropping weapon kill for %q", name)
	}
}

// Flush persists all pending changes immediately.
func (s *Store) Flush() {
	done := make(chan struct{})
	select {
	case s.flushCh <- done:
		<-done
	default:
	}
}

// Invalidate drops the cached account for a disconnecting player.
func (s *Store) Invalidate(ip, fp string) {
	key := ip
	if key == "" {
		return
	}
	s.cacheMu.Lock()
	delete(s.cache, key)
	s.cacheMu.Unlock()
}

func normalizeBanIP(value string) (string, error) {
	value = strings.TrimSpace(value)
	if host, _, err := net.SplitHostPort(value); err == nil {
		value = host
	}
	address, err := netip.ParseAddr(value)
	if err != nil || address.Zone() != "" {
		if err == nil {
			err = errors.New("IP zones are not supported")
		}
		return "", err
	}
	return address.Unmap().String(), nil
}

// AddIPBan persists a ban before publishing it to the in-memory admission cache.
func (s *Store) AddIPBan(ip, reason string) error {
	ip, err := normalizeBanIP(ip)
	if err != nil {
		return err
	}
	reason = strings.TrimSpace(reason)
	createdAt := time.Now().Unix()

	s.banMu.Lock()
	defer s.banMu.Unlock()
	if _, err := s.db.Exec(`INSERT INTO ip_bans(ip, reason, created_at) VALUES(?, ?, ?)
		ON CONFLICT(ip) DO UPDATE SET reason=excluded.reason, created_at=excluded.created_at`, ip, reason, createdAt); err != nil {
		return err
	}
	s.ipBans[ip] = IPBan{IP: ip, Reason: reason, CreatedAt: createdAt}
	return nil
}

// DeleteIPBan removes a persisted ban before evicting it from the admission cache.
func (s *Store) DeleteIPBan(ip string) error {
	ip, err := normalizeBanIP(ip)
	if err != nil {
		return err
	}

	s.banMu.Lock()
	defer s.banMu.Unlock()
	if _, err := s.db.Exec(`DELETE FROM ip_bans WHERE ip=?`, ip); err != nil {
		return err
	}
	delete(s.ipBans, ip)
	return nil
}

func (s *Store) IsIPBanned(ip string) bool {
	ip, err := normalizeBanIP(ip)
	if err != nil {
		return false
	}
	s.banMu.RLock()
	_, banned := s.ipBans[ip]
	s.banMu.RUnlock()
	return banned
}

func (s *Store) ListIPBans() []IPBan {
	s.banMu.RLock()
	out := make([]IPBan, 0, len(s.ipBans))
	for _, ban := range s.ipBans {
		out = append(out, ban)
	}
	s.banMu.RUnlock()
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt != out[j].CreatedAt {
			return out[i].CreatedAt > out[j].CreatedAt
		}
		return out[i].IP < out[j].IP
	})
	return out
}

func (s *Store) GetOrCreatePlayer(ip, fp, name string) string {
	// The experiment intentionally treats the public client IP as the account.
	// Fingerprints are retained only for backwards-compatible schema data and
	// never split one IP into multiple progression accounts.
	key := ip
	if key != "" {
		s.cacheMu.RLock()
		cached, ok := s.cache[key]
		s.cacheMu.RUnlock()
		if ok {
			return cached
		}

		var existingName string
		var err error
		err = s.db.QueryRow(`SELECT name FROM stats WHERE ip != '' AND ip = ? ORDER BY updated_at DESC LIMIT 1`, ip).Scan(&existingName)
		if err == nil && existingName != "" {
			// Backfill whichever of ip/fingerprint was missing so both stay
			// uniquely bound to this one account row.
			_, _ = s.db.Exec(`UPDATE stats SET fingerprint=CASE WHEN fingerprint='' THEN ?2 ELSE fingerprint END,
				ip=CASE WHEN ip='' THEN ?1 ELSE ip END WHERE name=?3`, ip, fp, existingName)
			s.cacheMu.Lock()
			s.cache[key] = existingName
			s.cacheMu.Unlock()
			return existingName
		}
	}

	var resolved string
	err := s.db.QueryRow(`INSERT INTO stats(name, ip, fingerprint, updated_at)
		VALUES(CASE WHEN EXISTS(SELECT 1 FROM stats WHERE name=?1)
			THEN ?1 || '-' || lower(hex(randomblob(6))) ELSE ?1 END,
			?2, ?3, strftime('%s','now')) RETURNING name`, name, ip, fp).Scan(&resolved)
	if err != nil {
		log.Printf("create player: %v", err)
		return name
	}
	if key != "" {
		s.cacheMu.Lock()
		s.cache[key] = resolved
		s.cacheMu.Unlock()
	}
	return resolved
}

func (s *Store) AccountForIP(ip string) (string, bool) {
	var name string
	err := s.db.QueryRow(`SELECT name FROM stats WHERE ip != '' AND ip=? ORDER BY updated_at DESC LIMIT 1`, ip).Scan(&name)
	return name, err == nil && name != ""
}

func (s *Store) WeaponProgressForIP(ip string) ([]WeaponProgress, error) {
	s.Flush()
	name, ok := s.AccountForIP(ip)
	if !ok {
		return []WeaponProgress{}, nil
	}
	return s.WeaponProgress(name)
}

func (s *Store) WeaponProgress(name string) ([]WeaponProgress, error) {
	rows, err := s.db.Query(`SELECT weapon, kills FROM weapon_kills WHERE account=? ORDER BY weapon`, name)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []WeaponProgress{}
	for rows.Next() {
		var p WeaponProgress
		if err := rows.Scan(&p.Weapon, &p.Kills); err != nil {
			return nil, err
		}
		p.Gold = p.Kills >= GoldKillRequirement
		p.Diamond = p.Kills >= DiamondKillRequirement
		out = append(out, p)
	}
	return out, rows.Err()
}

func (s *Store) UnlockedWeaponSkin(name string, weapon, requested uint8) uint8 {
	if requested == 0 {
		return 0
	}
	var kills int
	_ = s.db.QueryRow(`SELECT kills FROM weapon_kills WHERE account=? AND weapon=?`, name, weapon).Scan(&kills)
	if requested == 3 {
		maxSkin := 0
		if kills >= GoldKillRequirement {
			maxSkin = 1
		}
		if kills >= DiamondKillRequirement {
			maxSkin = 2
		}
		return uint8(rand.IntN(maxSkin + 1))
	}
	if requested == 2 && kills >= DiamondKillRequirement {
		return 2
	}
	if requested == 1 && kills >= GoldKillRequirement {
		return 1
	}
	return 0
}

type LeaderRow struct {
	Name   string  `json:"name"`
	Kills  uint32  `json:"kills"`
	Deaths uint32  `json:"deaths"`
	KD     float64 `json:"kd"`
}

func (s *Store) Leaderboard(n int) ([]LeaderRow, error) {
	// Flush pending in-memory batch before querying leaderboard
	done := make(chan struct{})
	select {
	case s.flushCh <- done:
		<-done
	default:
	}

	rows, err := s.db.Query(
		`SELECT name, kills, deaths FROM stats WHERE kills + deaths > 0
			AND name NOT LIKE '[BOT]%' AND name NOT LIKE 'load-%'
			AND name NOT IN ('VoxelKing','ShadowSniper','ApexGhost','ViperZero','Phoenix','Maverick','CyberWolf',
				'Soldier','VoxelMaster','BugHunter','ColTest','Commander','Tester','ApexHunter','General')
			ORDER BY kills DESC, deaths ASC LIMIT ?`, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LeaderRow{}
	for rows.Next() {
		var r LeaderRow
		var name sql.NullString
		var k, d int64
		if err := rows.Scan(&name, &k, &d); err != nil {
			log.Printf("leaderboard scan error: %v", err)
			continue
		}
		r.Name = name.String
		r.Kills = uint32(k)
		r.Deaths = uint32(d)
		if d == 0 {
			r.KD = float64(k)
		} else {
			r.KD = math.Round(float64(k)/float64(d)*100) / 100
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// IncrMeta atomically bumps a named counter and returns the new value.
func (s *Store) IncrMeta(key string, by int64) int64 {
	var v int64
	err := s.db.QueryRow(`INSERT INTO meta(key,val) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET val=val+excluded.val RETURNING val`, key, by).Scan(&v)
	if err != nil {
		return 0
	}
	return v
}

func (s *Store) SetMeta(key string, value int64) error {
	_, err := s.db.Exec(`INSERT INTO meta(key,val) VALUES(?,?)
		ON CONFLICT(key) DO UPDATE SET val=excluded.val`, key, value)
	return err
}

func (s *Store) GetMeta(key string) int64 {
	var v int64
	s.db.QueryRow(`SELECT val FROM meta WHERE key=?`, key).Scan(&v)
	return v
}

func (s *Store) Close() error {
	done := make(chan struct{})
	s.stopCh <- done
	<-done
	return s.db.Close()
}
