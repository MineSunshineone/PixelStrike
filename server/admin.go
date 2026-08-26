package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	adminCookieName = "pixelstrike_admin"
	adminSessionTTL = 8 * time.Hour
	adminLockout    = 15 * time.Minute
)

type loginAttempt struct {
	failures              int
	windowStart, lastSeen time.Time
	lockedUntil           time.Time
}

// ponytail: sessions and limits are process-local; use a shared store before running multiple server replicas.
type adminAuth struct {
	enabled      bool
	forceSecure  bool
	passwordHash [32]byte
	ips          IPResolver
	mu           sync.Mutex
	sessions     map[string]time.Time
	attempts     map[string]loginAttempt
}

func newAdminAuth(password string, ips IPResolver, forceSecure bool) *adminAuth {
	return &adminAuth{
		enabled:      password != "",
		forceSecure:  forceSecure,
		passwordHash: sha256.Sum256([]byte(password)),
		ips:          ips,
		sessions:     make(map[string]time.Time),
		attempts:     make(map[string]loginAttempt),
	}
}

func (a *adminAuth) login(w http.ResponseWriter, r *http.Request) {
	adminHeaders(w)
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		adminError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !sameOrigin(r) {
		adminError(w, http.StatusForbidden, "forbidden origin")
		return
	}
	if !a.enabled {
		adminError(w, http.StatusServiceUnavailable, "admin password is not configured")
		return
	}
	ip := a.ips.ClientIP(r)
	if ip == "" {
		adminError(w, http.StatusBadRequest, "invalid client address")
		return
	}
	now := time.Now()
	if retry := a.retryAfter(ip, now); retry > 0 {
		w.Header().Set("Retry-After", strconv.Itoa(max(1, int(retry.Round(time.Second)/time.Second))))
		adminError(w, http.StatusTooManyRequests, "too many login attempts")
		return
	}
	var request struct {
		Password string `json:"password"`
	}
	if !decodeAdminJSON(w, r, &request) {
		return
	}
	provided := sha256.Sum256([]byte(request.Password))
	if subtle.ConstantTimeCompare(provided[:], a.passwordHash[:]) != 1 {
		if retry := a.recordFailure(ip, now); retry > 0 {
			w.Header().Set("Retry-After", strconv.Itoa(int(retry/time.Second)))
			adminError(w, http.StatusTooManyRequests, "too many login attempts")
		} else {
			adminError(w, http.StatusUnauthorized, "login failed")
		}
		return
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		adminError(w, http.StatusInternalServerError, "session unavailable")
		return
	}
	token := base64.RawURLEncoding.EncodeToString(tokenBytes)
	expires := now.Add(adminSessionTTL)
	a.mu.Lock()
	delete(a.attempts, ip)
	for value, expiry := range a.sessions {
		if !now.Before(expiry) {
			delete(a.sessions, value)
		}
	}
	a.sessions[token] = expires
	a.mu.Unlock()
	http.SetCookie(w, &http.Cookie{
		Name: adminCookieName, Value: token, Path: "/api/admin", Expires: expires,
		MaxAge: int(adminSessionTTL / time.Second), HttpOnly: true, Secure: a.forceSecure || requestIsHTTPS(r, a.ips), SameSite: http.SameSiteStrictMode,
	})
	writeAdminJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (a *adminAuth) logout(w http.ResponseWriter, r *http.Request) {
	adminHeaders(w)
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", "POST")
		adminError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if !sameOrigin(r) {
		adminError(w, http.StatusForbidden, "forbidden origin")
		return
	}
	if cookie, err := r.Cookie(adminCookieName); err == nil {
		a.mu.Lock()
		delete(a.sessions, cookie.Value)
		a.mu.Unlock()
	}
	http.SetCookie(w, &http.Cookie{
		Name: adminCookieName, Path: "/api/admin", MaxAge: -1, Expires: time.Unix(1, 0),
		HttpOnly: true, Secure: a.forceSecure || requestIsHTTPS(r, a.ips), SameSite: http.SameSiteStrictMode,
	})
	w.WriteHeader(http.StatusNoContent)
}

func (a *adminAuth) require(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		adminHeaders(w)
		if r.Method != http.MethodGet && !sameOrigin(r) {
			adminError(w, http.StatusForbidden, "forbidden origin")
			return
		}
		cookie, err := r.Cookie(adminCookieName)
		if err != nil {
			adminError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		now := time.Now()
		a.mu.Lock()
		expires, ok := a.sessions[cookie.Value]
		if ok && !now.Before(expires) {
			delete(a.sessions, cookie.Value)
			ok = false
		}
		a.mu.Unlock()
		if !ok {
			adminError(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func (a *adminAuth) retryAfter(ip string, now time.Time) time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	attempt, ok := a.attempts[ip]
	if !ok {
		return 0
	}
	if now.Before(attempt.lockedUntil) {
		return attempt.lockedUntil.Sub(now)
	}
	if !attempt.lockedUntil.IsZero() || now.Sub(attempt.windowStart) > 10*time.Minute {
		delete(a.attempts, ip)
	}
	return 0
}

func (a *adminAuth) recordFailure(ip string, now time.Time) time.Duration {
	a.mu.Lock()
	defer a.mu.Unlock()
	if len(a.attempts) >= 4096 {
		for key, attempt := range a.attempts {
			if now.Sub(attempt.lastSeen) > 30*time.Minute {
				delete(a.attempts, key)
			}
		}
		if _, exists := a.attempts[ip]; !exists && len(a.attempts) >= 4096 {
			return adminLockout
		}
	}
	attempt := a.attempts[ip]
	if attempt.windowStart.IsZero() || now.Sub(attempt.windowStart) > 10*time.Minute {
		attempt = loginAttempt{windowStart: now}
	}
	attempt.failures++
	attempt.lastSeen = now
	if attempt.failures >= 5 {
		attempt.lockedUntil = now.Add(adminLockout)
	}
	a.attempts[ip] = attempt
	if now.Before(attempt.lockedUntil) {
		return attempt.lockedUntil.Sub(now)
	}
	return 0
}

func registerAdminHandlers(mux *http.ServeMux, hub *Hub, store *Store, password string, ips IPResolver, forceSecure bool) {
	auth := newAdminAuth(password, ips, forceSecure)
	mux.HandleFunc("/api/admin/login", auth.login)
	mux.HandleFunc("/api/admin/logout", auth.logout)
	mux.HandleFunc("/api/admin/overview", auth.require(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			w.Header().Set("Allow", "GET")
			adminError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		result := runtimeStats(hub, store)
		bots, _ := hub.BotStatus()
		result["bots"] = bots
		result["players"] = hub.OnlineSnapshot()
		result["bans"] = store.ListIPBans()
		writeAdminJSON(w, http.StatusOK, result)
	}))
	mux.HandleFunc("/api/admin/bots", auth.require(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			count, rooms := hub.BotStatus()
			writeAdminJSON(w, http.StatusOK, map[string]int{"bots": count, "rooms": rooms})
		case http.MethodPost:
			var request struct {
				Bots int `json:"bots"`
			}
			if !decodeAdminJSON(w, r, &request) {
				return
			}
			if request.Bots < 0 || request.Bots > len(BotNames) {
				adminError(w, http.StatusBadRequest, "bots must be an integer from 0 to 12")
				return
			}
			count, rooms, err := hub.SetBotCount(request.Bots)
			if err != nil {
				adminError(w, http.StatusInternalServerError, "failed to save bot setting")
				return
			}
			writeAdminJSON(w, http.StatusOK, map[string]int{"bots": count, "rooms": rooms})
		default:
			w.Header().Set("Allow", "GET, POST")
			adminError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	}))
	mux.HandleFunc("/api/admin/bans", auth.require(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			writeAdminJSON(w, http.StatusOK, store.ListIPBans())
			return
		}
		if r.Method != http.MethodPost && r.Method != http.MethodDelete {
			w.Header().Set("Allow", "GET, POST, DELETE")
			adminError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		var request struct {
			IP     string `json:"ip"`
			Reason string `json:"reason"`
		}
		if !decodeAdminJSON(w, r, &request) {
			return
		}
		address, err := netip.ParseAddr(strings.TrimSpace(request.IP))
		if err != nil || address.Zone() != "" {
			adminError(w, http.StatusBadRequest, "invalid IP address")
			return
		}
		ip := address.Unmap().String()
		if r.Method == http.MethodDelete {
			if err := store.DeleteIPBan(ip); err != nil {
				adminError(w, http.StatusInternalServerError, "database error")
				return
			}
			writeAdminJSON(w, http.StatusOK, map[string]any{"ip": ip, "unbanned": true})
			return
		}
		reason := strings.TrimSpace(request.Reason)
		if len([]rune(reason)) > 120 {
			adminError(w, http.StatusBadRequest, "reason is too long")
			return
		}
		if reason == "" {
			reason = "manual"
		}
		kicked, err := hub.BanIP(ip, reason)
		if err != nil {
			adminError(w, http.StatusInternalServerError, "database error")
			return
		}
		writeAdminJSON(w, http.StatusOK, map[string]any{"ip": ip, "banned": true, "kicked": kicked})
	}))
}

func decodeAdminJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		adminError(w, http.StatusUnsupportedMediaType, "application/json required")
		return false
	}
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil || decoder.Decode(&struct{}{}) != io.EOF {
		adminError(w, http.StatusBadRequest, "invalid request")
		return false
	}
	return true
}

func requestIsHTTPS(r *http.Request, ips IPResolver) bool {
	if r.TLS != nil {
		return true
	}
	peer, ok := parseRequestIP(r.RemoteAddr)
	if !ok || !ips.trusted(peer) {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]), "https")
}

func adminHeaders(w http.ResponseWriter) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("X-Content-Type-Options", "nosniff")
}

func adminError(w http.ResponseWriter, status int, message string) {
	writeAdminJSON(w, status, map[string]string{"error": message})
}

func writeAdminJSON(w http.ResponseWriter, status int, value any) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
