package home

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/netip"
	"slices"
	"strconv"
	"strings"
	"sync"

	"github.com/AdguardTeam/AdGuardHome/internal/aghhttp"
)

const (
	defaultGameListURL = "https://raw.githubusercontent.com/JosuhaSanhueza/BlockList/refs/heads/main/GamesBlockList.txt"
	defaultStartIP     = "192.168.12.101"
	defaultEndIP       = "192.168.12.145"
	defaultLabID       = "lab1"
	defaultLabName     = "Laboratorio 1"

	// maxLabHosts is the maximum number of hosts a single lab range may cover.
	maxLabHosts = 512
)

// GameControlHost represents an individual host state within a lab.
type GameControlHost struct {
	IP   string `json:"ip"`
	Host string `json:"host"`
	// Blocked reflects the games/entertainment blocklist for this host.
	Blocked bool `json:"blocked"`
	// InternetBlocked is true if this host's IP is in the DNS access
	// settings' disallowed-clients list, meaning it has no DNS resolution
	// at all.  See [handleGameControlInternetToggleHost].
	InternetBlocked bool `json:"internet_blocked"`
}

// GameControlLab is a named group of machines defined by an IPv4 range.
type GameControlLab struct {
	ID         string `json:"id" yaml:"id"`
	Name       string `json:"name" yaml:"name"`
	RangeStart string `json:"range_start" yaml:"range_start"`
	RangeEnd   string `json:"range_end" yaml:"range_end"`
}

// GameControlConfig represents the settings and current status for GameControl.
type GameControlConfig struct {
	Enabled     bool   `json:"enabled" yaml:"enabled"`
	UpstreamURL string `json:"upstream_url" yaml:"upstream_url"`

	// Labs are the configured labs.
	Labs []GameControlLab `json:"labs" yaml:"labs"`

	// RangeStart and RangeEnd are the legacy single-lab range, migrated into
	// Labs on load and left empty afterwards.
	RangeStart string `json:"-" yaml:"range_start,omitempty"`
	RangeEnd   string `json:"-" yaml:"range_end,omitempty"`

	BlockedHosts map[string]bool `json:"blocked_hosts" yaml:"blocked_hosts"` // IP -> blocked state
}

type gameControlManager struct {
	mu     sync.RWMutex
	conf   GameControlConfig
	webReg aghhttp.Registrar

	// persist saves the configuration to disk.  It must be called without mu
	// held.  It may be nil, e.g. in tests.
	persist func(ctx context.Context)
}

var gameControlgameControlMgr = &gameControlManager{
	conf: GameControlConfig{
		Enabled:     true,
		UpstreamURL: defaultGameListURL,
		Labs: []GameControlLab{{
			ID:         defaultLabID,
			Name:       defaultLabName,
			RangeStart: defaultStartIP,
			RangeEnd:   defaultEndIP,
		}},
		BlockedHosts: make(map[string]bool),
	},
}

// save persists the configuration if possible.  It must be called without
// m.mu held.
func (m *gameControlManager) save(ctx context.Context) {
	if m.persist != nil {
		m.persist(ctx)
	}
}

// migrateLegacyRange converts the legacy single range into a lab.  It must be
// called with m.mu held for writing.
func (m *gameControlManager) migrateLegacyRange() {
	if m.conf.RangeStart == "" && m.conf.RangeEnd == "" {
		return
	}

	if len(m.conf.Labs) == 0 {
		m.conf.Labs = []GameControlLab{{
			ID:         defaultLabID,
			Name:       defaultLabName,
			RangeStart: m.conf.RangeStart,
			RangeEnd:   m.conf.RangeEnd,
		}}
	}

	m.conf.RangeStart = ""
	m.conf.RangeEnd = ""
}

func (web *webAPI) initGameControl() {
	webReg := web.httpReg
	gameControlgameControlMgr.webReg = webReg
	gameControlgameControlMgr.persist = web.confModifier.Apply

	webReg.Register(http.MethodGet, "/control/gamecontrol/status", handleGameControlStatus)
	webReg.Register(http.MethodPost, "/control/gamecontrol/update_host", handleGameControlUpdateHost)
	webReg.Register(http.MethodPost, "/control/gamecontrol/toggle_all", handleGameControlToggleAll)
	webReg.Register(http.MethodPost, "/control/gamecontrol/config", handleGameControlUpdateConfig)
	webReg.Register(
		http.MethodPost,
		"/control/gamecontrol/internet/toggle_host",
		handleGameControlInternetToggleHost,
	)
	webReg.Register(
		http.MethodPost,
		"/control/gamecontrol/internet/toggle_all",
		handleGameControlInternetToggleAll,
	)

	web.registerLabHandlers()
}

// labScope returns the lab ID the request's user is restricted to.  restricted
// is false for administrators.
func labScope(ctx context.Context) (labID string, restricted bool) {
	u, ok := webUserFromContext(ctx)
	if !ok || !u.IsTeacher() {
		return "", false
	}

	return u.LabID, true
}

func parseIP4(ipStr string) (uint32, bool) {
	addr, err := netip.ParseAddr(ipStr)
	if err != nil || !addr.Is4() {
		return 0, false
	}
	b := addr.As4()
	return uint32(b[0])<<24 | uint32(b[1])<<16 | uint32(b[2])<<8 | uint32(b[3]), true
}

func formatIP4(val uint32) string {
	return netip.AddrFrom4([4]byte{
		byte(val >> 24),
		byte(val >> 16),
		byte(val >> 8),
		byte(val),
	}).String()
}

// labHosts returns the hosts of lab.  It must be called with m.mu held.
func (m *gameControlManager) labHosts(lab GameControlLab) []GameControlHost {
	startVal, ok1 := parseIP4(lab.RangeStart)
	endVal, ok2 := parseIP4(lab.RangeEnd)
	if !ok1 || !ok2 || startVal > endVal || endVal-startVal >= maxLabHosts {
		return nil
	}

	hosts := make([]GameControlHost, 0, endVal-startVal+1)
	for i := startVal; i <= endVal; i++ {
		ip := formatIP4(i)

		blocked, exists := m.conf.BlockedHosts[ip]
		if !exists {
			blocked = true
		}

		hosts = append(hosts, GameControlHost{
			IP:      ip,
			Host:    "PC" + strconv.Itoa(int(i-startVal+1)),
			Blocked: blocked,
		})
	}

	return hosts
}

// scopedLabs returns a copy of the labs the request may see: all for
// administrators, only the assigned one for teachers.  It must be called with
// m.mu held.
func (m *gameControlManager) scopedLabs(ctx context.Context) (labs []GameControlLab) {
	labID, restricted := labScope(ctx)
	if !restricted {
		return slices.Clone(m.conf.Labs)
	}

	for _, l := range m.conf.Labs {
		if l.ID == labID {
			return []GameControlLab{l}
		}
	}

	return nil
}

// scopedLabsByID is like scopedLabs, but narrows the result to labID if it is
// not empty.  ok is false if labID was given and is not visible to the user.
func (m *gameControlManager) scopedLabsByID(
	ctx context.Context,
	labID string,
) (labs []GameControlLab, ok bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	labs = m.scopedLabs(ctx)
	if labID == "" {
		return labs, true
	}

	labs = slices.DeleteFunc(labs, func(l GameControlLab) bool { return l.ID != labID })

	return labs, len(labs) > 0
}

// hostsOf returns the hosts of all of the given labs.
func (m *gameControlManager) hostsOf(labs []GameControlLab) (hosts []GameControlHost) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, l := range labs {
		hosts = append(hosts, m.labHosts(l)...)
	}

	return hosts
}

// ipAllowed returns true if the request's user may control the given IP: it
// must belong to a lab visible to the user.
func (m *gameControlManager) ipAllowed(ctx context.Context, ip string) (ok bool) {
	labs, _ := m.scopedLabsByID(ctx, "")
	for _, h := range m.hostsOf(labs) {
		if h.IP == ip {
			return true
		}
	}

	return false
}

type gameControlLabResp struct {
	GameControlLab
	Hosts []GameControlHost `json:"hosts"`
}

type gameControlStatusResp struct {
	Enabled     bool                 `json:"enabled"`
	UpstreamURL string               `json:"upstream_url"`
	Restricted  bool                 `json:"restricted"`
	Labs        []gameControlLabResp `json:"labs"`
}

func handleGameControlStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	m := gameControlgameControlMgr

	labs, _ := m.scopedLabsByID(ctx, "")
	_, restricted := labScope(ctx)

	m.mu.RLock()
	resp := gameControlStatusResp{
		Enabled:     m.conf.Enabled,
		UpstreamURL: m.conf.UpstreamURL,
		Restricted:  restricted,
		Labs:        make([]gameControlLabResp, 0, len(labs)),
	}
	for _, l := range labs {
		resp.Labs = append(resp.Labs, gameControlLabResp{GameControlLab: l, Hosts: m.labHosts(l)})
	}
	m.mu.RUnlock()

	if dnsServer := globalContext.dnsServer; dnsServer != nil {
		disallowed := dnsServer.DisallowedClients()
		for _, l := range resp.Labs {
			for i, h := range l.Hosts {
				l.Hosts[i].InternetBlocked = slices.Contains(disallowed, h.IP)
			}
		}
	}

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, resp)
}

// internetToggleHostReq is the request body for
// POST /control/gamecontrol/internet/toggle_host.
type internetToggleHostReq struct {
	IP      string `json:"ip"`
	Blocked bool   `json:"blocked"`
}

// handleGameControlInternetToggleHost adds or removes a single host's IP
// from the DNS access settings' disallowed-clients list, which drops all of
// its DNS queries (not just games/entertainment).  This is meant as a quick,
// reversible way to cut a misbehaving lab PC off the internet entirely,
// without leaving the GameControl page.
func handleGameControlInternetToggleHost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req := &internetToggleHostReq{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "invalid request: %s", err)

		return
	}

	if !gameControlgameControlMgr.ipAllowed(ctx, req.IP) {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusForbidden, "host is not in your lab")

		return
	}

	dnsServer := globalContext.dnsServer
	if dnsServer == nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusServiceUnavailable, "dns server is not ready")

		return
	}

	clients := dnsServer.DisallowedClients()
	isDisallowed := slices.Contains(clients, req.IP)

	switch {
	case req.Blocked && !isDisallowed:
		clients = append(clients, req.IP)
	case !req.Blocked && isDisallowed:
		clients = slices.DeleteFunc(clients, func(c string) bool { return c == req.IP })
	default:
		// Already in the desired state; nothing to do.
		aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, map[string]string{"result": "ok"})

		return
	}

	if err := dnsServer.SetDisallowedClients(ctx, clients); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusInternalServerError, "%s", err)

		return
	}

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, map[string]string{"result": "ok"})
}

// internetToggleAllReq is the request body for
// POST /control/gamecontrol/internet/toggle_all.  An empty LabID means every
// lab the user can see.
type internetToggleAllReq struct {
	LabID   string `json:"lab_id"`
	Blocked bool   `json:"blocked"`
}

// handleGameControlInternetToggleAll adds or removes every host of the
// requested lab from the DNS access settings' disallowed-clients list in a
// single update, cutting off (or restoring) the whole lab's internet access at
// once.
func handleGameControlInternetToggleAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	req := &internetToggleAllReq{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "invalid request: %s", err)

		return
	}

	m := gameControlgameControlMgr
	labs, ok := m.scopedLabsByID(ctx, req.LabID)
	if !ok {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusForbidden, "lab not available")

		return
	}

	dnsServer := globalContext.dnsServer
	if dnsServer == nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusServiceUnavailable, "dns server is not ready")

		return
	}

	clients := withRangeDisallowed(dnsServer.DisallowedClients(), m.hostsOf(labs), req.Blocked)

	if err := dnsServer.SetDisallowedClients(ctx, clients); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusInternalServerError, "%s", err)

		return
	}

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, map[string]string{"result": "ok"})
}

// withRangeDisallowed returns clients with every host in rangeHosts added
// (blocked=true) or removed (blocked=false).
func withRangeDisallowed(clients []string, rangeHosts []GameControlHost, blocked bool) []string {
	if blocked {
		for _, h := range rangeHosts {
			if !slices.Contains(clients, h.IP) {
				clients = append(clients, h.IP)
			}
		}

		return clients
	}

	rangeIPs := make(map[string]bool, len(rangeHosts))
	for _, h := range rangeHosts {
		rangeIPs[h.IP] = true
	}

	return slices.DeleteFunc(clients, func(c string) bool { return rangeIPs[c] })
}

type updateHostReq struct {
	IP      string `json:"ip"`
	Blocked bool   `json:"blocked"`
}

func handleGameControlUpdateHost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req updateHostReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "invalid request: %s", err)
		return
	}

	m := gameControlgameControlMgr
	if !m.ipAllowed(ctx, req.IP) {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusForbidden, "host is not in your lab")

		return
	}

	m.mu.Lock()
	if m.conf.BlockedHosts == nil {
		m.conf.BlockedHosts = make(map[string]bool)
	}
	m.conf.BlockedHosts[req.IP] = req.Blocked
	m.mu.Unlock()

	m.save(ctx)

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, map[string]string{"result": "ok"})
}

type toggleAllReq struct {
	LabID   string `json:"lab_id"`
	Blocked bool   `json:"blocked"`
}

func handleGameControlToggleAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req toggleAllReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "invalid request: %s", err)
		return
	}

	m := gameControlgameControlMgr
	labs, ok := m.scopedLabsByID(ctx, req.LabID)
	if !ok {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusForbidden, "lab not available")

		return
	}

	hosts := m.hostsOf(labs)

	m.mu.Lock()
	if m.conf.BlockedHosts == nil {
		m.conf.BlockedHosts = make(map[string]bool)
	}
	for _, h := range hosts {
		m.conf.BlockedHosts[h.IP] = req.Blocked
	}
	m.mu.Unlock()

	m.save(ctx)

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, map[string]string{"result": "ok"})
}

type updateConfigReq struct {
	Enabled     *bool  `json:"enabled,omitempty"`
	UpstreamURL string `json:"upstream_url,omitempty"`
}

// handleGameControlUpdateConfig updates the global module settings.  It is
// administrator-only.
func handleGameControlUpdateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if _, restricted := labScope(ctx); restricted {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusForbidden, "administrators only")

		return
	}

	var req updateConfigReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "invalid request: %s", err)
		return
	}

	m := gameControlgameControlMgr
	m.mu.Lock()
	if req.Enabled != nil {
		m.conf.Enabled = *req.Enabled
	}
	if strings.TrimSpace(req.UpstreamURL) != "" {
		m.conf.UpstreamURL = strings.TrimSpace(req.UpstreamURL)
	}
	m.mu.Unlock()

	m.save(ctx)

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, map[string]string{"result": "ok"})
}

// IsIPGameAllowed checks if a given IP address is explicitly allowed (unblocked) in GameControl.
func IsIPGameAllowed(ipStr string) bool {
	gameControlgameControlMgr.mu.RLock()
	defer gameControlgameControlMgr.mu.RUnlock()

	if !gameControlgameControlMgr.conf.Enabled {
		return true
	}
	// Default is blocked = true unless BlockedHosts[ipStr] is set to false (unblocked)
	isBlocked, exists := gameControlgameControlMgr.conf.BlockedHosts[ipStr]
	if !exists {
		return false // Blocked by default
	}
	return !isBlocked // Allowed if blocked == false
}

// IsGameDomain checks if a domain belongs to common game blocklists or Poki.
func IsGameDomain(host string) bool {
	h := strings.ToLower(host)
	return strings.Contains(h, "poki.com") ||
		strings.Contains(h, "roblox.com") ||
		strings.Contains(h, "epicgames.com") ||
		strings.Contains(h, "steamcommunity.com") ||
		strings.Contains(h, "steampowered.com") ||
		strings.Contains(h, "riotgames.com")
}
