package main

import (
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func adminLoginRequest(ip, password string) *http.Request {
	r := httptest.NewRequest(http.MethodPost, "https://admin.example/api/admin/login", strings.NewReader(`{"password":"`+password+`"}`))
	r.RemoteAddr = ip + ":4242"
	r.Header.Set("Content-Type", "application/json")
	r.Header.Set("Origin", "https://admin.example")
	return r
}

func TestIPResolverUsesForwardedIPOnlyThroughTrustedProxy(t *testing.T) {
	resolver, err := NewIPResolver("10.0.0.0/8")
	if err != nil {
		t.Fatal(err)
	}

	untrusted := httptest.NewRequest(http.MethodGet, "http://game.example/", nil)
	untrusted.RemoteAddr = "198.51.100.10:1234"
	untrusted.Header.Set("X-Forwarded-For", "203.0.113.10")
	if got := resolver.ClientIP(untrusted); got != "198.51.100.10" {
		t.Fatalf("untrusted forwarded IP = %q, want peer IP", got)
	}

	trusted := httptest.NewRequest(http.MethodGet, "http://game.example/", nil)
	trusted.RemoteAddr = "10.0.0.2:1234"
	trusted.Header.Set("X-Forwarded-For", "203.0.113.10, 10.0.0.3")
	trusted.Header.Set("X-Real-IP", "198.51.100.20")
	if got := resolver.ClientIP(trusted); got != "203.0.113.10" {
		t.Fatalf("trusted forwarded IP = %q, want first untrusted hop", got)
	}

	trusted.Header.Set("X-Forwarded-For", "10.0.0.3")
	if got := resolver.ClientIP(trusted); got != "198.51.100.20" {
		t.Fatalf("trusted X-Real-IP = %q, want 198.51.100.20", got)
	}

	ipv6 := httptest.NewRequest(http.MethodGet, "http://game.example/", nil)
	ipv6.RemoteAddr = "[2001:db8::10]:1234"
	if got := resolver.ClientIP(ipv6); got != "2001:db8::10" {
		t.Fatalf("IPv6 peer = %q, want canonical IPv6", got)
	}
}

func TestAdminAuthLocksPerIPAndProtectsSession(t *testing.T) {
	auth := newAdminAuth("correct-password", nil, false)

	for i := 0; i < 4; i++ {
		rr := httptest.NewRecorder()
		auth.login(rr, adminLoginRequest("203.0.113.10", "wrong"))
		if rr.Code != http.StatusUnauthorized {
			t.Fatalf("failed login %d status = %d, want 401", i+1, rr.Code)
		}
	}
	locked := httptest.NewRecorder()
	auth.login(locked, adminLoginRequest("203.0.113.10", "wrong"))
	if locked.Code != http.StatusTooManyRequests {
		t.Fatalf("fifth failed login status = %d, want 429", locked.Code)
	}
	if locked.Header().Get("Retry-After") == "" {
		t.Fatal("locked login did not return Retry-After")
	}

	// Lockout is keyed by the resolved IP, not global to all administrators.
	login := httptest.NewRecorder()
	auth.login(login, adminLoginRequest("203.0.113.11", "correct-password"))
	if login.Code != http.StatusOK {
		t.Fatalf("valid login from another IP status = %d", login.Code)
	}
	cookies := login.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("login returned %d cookies, want one", len(cookies))
	}
	cookie := cookies[0]
	if cookie.Name != adminCookieName || cookie.Value == "" || !cookie.HttpOnly || !cookie.Secure || cookie.SameSite != http.SameSiteStrictMode || cookie.Path != "/api/admin" {
		t.Fatalf("unsafe session cookie: %+v", cookie)
	}

	protected := auth.require(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})
	request := httptest.NewRequest(http.MethodGet, "https://admin.example/api/admin/overview", nil)
	request.RemoteAddr = "203.0.113.11:4242"
	request.AddCookie(cookie)
	response := httptest.NewRecorder()
	protected(response, request)
	if response.Code != http.StatusNoContent {
		t.Fatalf("valid session status = %d, want 204", response.Code)
	}

	csrf := httptest.NewRequest(http.MethodPost, "https://admin.example/api/admin/bans", nil)
	csrf.RemoteAddr = "203.0.113.11:4242"
	csrf.Header.Set("Origin", "https://evil.example")
	csrf.AddCookie(cookie)
	response = httptest.NewRecorder()
	protected(response, csrf)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin mutation status = %d, want 403", response.Code)
	}

	logout := httptest.NewRequest(http.MethodPost, "https://admin.example/api/admin/logout", nil)
	logout.RemoteAddr = "203.0.113.11:4242"
	logout.Header.Set("Origin", "https://admin.example")
	logout.AddCookie(cookie)
	logoutResponse := httptest.NewRecorder()
	auth.logout(logoutResponse, logout)
	if logoutResponse.Code != http.StatusNoContent || logoutResponse.Result().Cookies()[0].MaxAge != -1 {
		t.Fatalf("logout response = %d, cookie = %+v", logoutResponse.Code, logoutResponse.Result().Cookies())
	}
	response = httptest.NewRecorder()
	protected(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("revoked session status = %d, want 401", response.Code)
	}
}

func TestIPBanPersistsAndRejectsWebSocketBeforeUpgrade(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AddIPBan("203.0.113.20", "test ban"); err != nil {
		store.Close()
		t.Fatal(err)
	}
	if !store.IsIPBanned("203.0.113.20:443") {
		store.Close()
		t.Fatal("newly added IP ban was not admitted")
	}
	if bans := store.ListIPBans(); len(bans) != 1 || bans[0].IP != "203.0.113.20" {
		store.Close()
		t.Fatalf("stored bans = %+v", bans)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if !store.IsIPBanned("203.0.113.20") {
		t.Fatal("IP ban did not survive store reopen")
	}

	resolver, err := NewIPResolver("")
	if err != nil {
		t.Fatal(err)
	}
	hub := &Hub{Store: store}
	request := httptest.NewRequest(http.MethodGet, "http://game.example/ws", nil)
	request.RemoteAddr = "203.0.113.20:4242"
	response := httptest.NewRecorder()
	ServeWS(hub, response, request, "", resolver)
	if response.Code != http.StatusForbidden {
		t.Fatalf("banned WebSocket status = %d, want 403 before upgrade", response.Code)
	}
}

func TestBotCountPersistsAcrossRestart(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "stats.db")
	store, err := NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub(nil, store)
	if bots, _ := hub.BotStatus(); bots != 8 {
		t.Fatalf("default bots = %d, want 8", bots)
	}
	if bots, _, err := hub.SetBotCount(3); err != nil || bots != 3 {
		t.Fatalf("set bots = %d, err = %v", bots, err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	store, err = NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if bots, _ := NewHub(nil, store).BotStatus(); bots != 3 {
		t.Fatalf("restored bots = %d, want 3", bots)
	}
}

func TestServeWSHonorsConfiguredOrigin(t *testing.T) {
	hub := &Hub{Store: &Store{ipBans: make(map[string]IPBan)}}
	request := httptest.NewRequest(http.MethodGet, "http://game.example/ws", nil)
	request.RemoteAddr = "203.0.113.30:4242"
	request.Header.Set("Origin", "https://evil.example")
	request.Header.Set("Connection", "Upgrade")
	request.Header.Set("Upgrade", "websocket")
	request.Header.Set("Sec-WebSocket-Version", "13")
	request.Header.Set("Sec-WebSocket-Key", "dGhlIHNhbXBsZSBub25jZQ==")
	response := httptest.NewRecorder()
	ServeWS(hub, response, request, "https://game.example", nil)
	if response.Code != http.StatusForbidden {
		t.Fatalf("cross-origin WebSocket status = %d, want 403", response.Code)
	}
}

func TestKickIPNormalizesAddress(t *testing.T) {
	player := &Player{IP: "203.0.113.40"}
	hub := &Hub{rooms: []*Room{{Players: []*Player{player}}}}
	if kicked := hub.KickIP("203.0.113.40:4242"); kicked != 1 {
		t.Fatalf("kicked = %d, want 1", kicked)
	}
}
