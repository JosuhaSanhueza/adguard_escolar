package home

import (
	"encoding/json"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"

	"github.com/AdguardTeam/AdGuardHome/internal/aghhttp"
)

const (
	defaultGameListURL = "https://raw.githubusercontent.com/JosuhaSanhueza/BlockList/refs/heads/main/GamesBlockList.txt"
	defaultStartIP     = "192.168.12.101"
	defaultEndIP       = "192.168.12.145"
)

// GameControlHost represents an individual host state within the GameControl range.
type GameControlHost struct {
	IP      string `json:"ip"`
	Host    string `json:"host"`
	Blocked bool   `json:"blocked"`
}

// GameControlConfig represents the settings and current status for GameControl.
type GameControlConfig struct {
	Enabled      bool            `json:"enabled" yaml:"enabled"`
	UpstreamURL  string          `json:"upstream_url" yaml:"upstream_url"`
	RangeStart   string          `json:"range_start" yaml:"range_start"`
	RangeEnd     string          `json:"range_end" yaml:"range_end"`
	BlockedHosts map[string]bool `json:"blocked_hosts" yaml:"blocked_hosts"` // IP -> blocked state
}

type gameControlManager struct {
	mu     sync.RWMutex
	conf   GameControlConfig
	webReg aghhttp.Registrar
}

var gameControlgameControlMgr = &gameControlManager{
	conf: GameControlConfig{
		Enabled:      true,
		UpstreamURL:  defaultGameListURL,
		RangeStart:   defaultStartIP,
		RangeEnd:     defaultEndIP,
		BlockedHosts: make(map[string]bool),
	},
}

func initGameControl(webReg aghhttp.Registrar) {
	gameControlgameControlMgr.webReg = webReg

	webReg.Register(http.MethodGet, "/control/gamecontrol/status", handleGameControlStatus)
	webReg.Register(http.MethodPost, "/control/gamecontrol/update_host", handleGameControlUpdateHost)
	webReg.Register(http.MethodPost, "/control/gamecontrol/toggle_all", handleGameControlToggleAll)
	webReg.Register(http.MethodPost, "/control/gamecontrol/config", handleGameControlUpdateConfig)
}

type gameControlStatusResp struct {
	Enabled     bool              `json:"enabled"`
	UpstreamURL string            `json:"upstream_url"`
	RangeStart  string            `json:"range_start"`
	RangeEnd    string            `json:"range_end"`
	Hosts       []GameControlHost `json:"hosts"`
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

func (m *gameControlManager) getHosts() []GameControlHost {
	m.mu.RLock()
	defer m.mu.RUnlock()

	startVal, ok1 := parseIP4(m.conf.RangeStart)
	endVal, ok2 := parseIP4(m.conf.RangeEnd)

	if !ok1 || !ok2 || startVal > endVal {
		startVal, _ = parseIP4(defaultStartIP)
		endVal, _ = parseIP4(defaultEndIP)
	}

	hosts := make([]GameControlHost, 0, endVal-startVal+1)
	for i := startVal; i <= endVal; i++ {
		ip := formatIP4(i)
		pcNum := i - startVal + 1
		hostName := "PC" + strconv.Itoa(int(pcNum))

		blocked, exists := m.conf.BlockedHosts[ip]
		if !exists {
			blocked = false
		}

		hosts = append(hosts, GameControlHost{
			IP:      ip,
			Host:    hostName,
			Blocked: blocked,
		})
	}
	return hosts
}

func handleGameControlStatus(w http.ResponseWriter, r *http.Request) {
	gameControlgameControlMgr.mu.RLock()
	resp := gameControlStatusResp{
		Enabled:     gameControlgameControlMgr.conf.Enabled,
		UpstreamURL: gameControlgameControlMgr.conf.UpstreamURL,
		RangeStart:  gameControlgameControlMgr.conf.RangeStart,
		RangeEnd:    gameControlgameControlMgr.conf.RangeEnd,
		Hosts:       gameControlgameControlMgr.getHosts(),
	}
	gameControlgameControlMgr.mu.RUnlock()

	aghhttp.WriteJSONResponseOK(r.Context(), nil, w, r, resp)
}

type updateHostReq struct {
	IP      string `json:"ip"`
	Blocked bool   `json:"blocked"`
}

func handleGameControlUpdateHost(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req updateHostReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aghhttp.ErrorAndLog(ctx, nil, r, w, http.StatusBadRequest, "invalid request: %s", err)
		return
	}

	gameControlgameControlMgr.mu.Lock()
	if gameControlgameControlMgr.conf.BlockedHosts == nil {
		gameControlgameControlMgr.conf.BlockedHosts = make(map[string]bool)
	}
	gameControlgameControlMgr.conf.BlockedHosts[req.IP] = req.Blocked
	gameControlgameControlMgr.mu.Unlock()

	aghhttp.WriteJSONResponseOK(ctx, nil, w, r, map[string]string{"result": "ok"})
}

type toggleAllReq struct {
	Blocked bool `json:"blocked"`
}

func handleGameControlToggleAll(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req toggleAllReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aghhttp.ErrorAndLog(ctx, nil, r, w, http.StatusBadRequest, "invalid request: %s", err)
		return
	}

	hosts := gameControlgameControlMgr.getHosts()

	gameControlgameControlMgr.mu.Lock()
	if gameControlgameControlMgr.conf.BlockedHosts == nil {
		gameControlgameControlMgr.conf.BlockedHosts = make(map[string]bool)
	}
	for _, h := range hosts {
		gameControlgameControlMgr.conf.BlockedHosts[h.IP] = req.Blocked
	}
	gameControlgameControlMgr.mu.Unlock()

	aghhttp.WriteJSONResponseOK(ctx, nil, w, r, map[string]string{"result": "ok"})
}

type updateConfigReq struct {
	Enabled     *bool  `json:"enabled,omitempty"`
	UpstreamURL string `json:"upstream_url,omitempty"`
	RangeStart  string `json:"range_start,omitempty"`
	RangeEnd    string `json:"range_end,omitempty"`
}

func handleGameControlUpdateConfig(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	var req updateConfigReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		aghhttp.ErrorAndLog(ctx, nil, r, w, http.StatusBadRequest, "invalid request: %s", err)
		return
	}

	gameControlgameControlMgr.mu.Lock()
	if req.Enabled != nil {
		gameControlgameControlMgr.conf.Enabled = *req.Enabled
	}
	if strings.TrimSpace(req.UpstreamURL) != "" {
		gameControlgameControlMgr.conf.UpstreamURL = strings.TrimSpace(req.UpstreamURL)
	}
	if strings.TrimSpace(req.RangeStart) != "" {
		gameControlgameControlMgr.conf.RangeStart = strings.TrimSpace(req.RangeStart)
	}
	if strings.TrimSpace(req.RangeEnd) != "" {
		gameControlgameControlMgr.conf.RangeEnd = strings.TrimSpace(req.RangeEnd)
	}
	gameControlgameControlMgr.mu.Unlock()

	aghhttp.WriteJSONResponseOK(ctx, nil, w, r, map[string]string{"result": "ok"})
}

// IsIPGameBlocked checks if a given IP address has GameControl blocking active.
func IsIPGameBlocked(ipStr string) bool {
	gameControlgameControlMgr.mu.RLock()
	defer gameControlgameControlMgr.mu.RUnlock()

	if !gameControlgameControlMgr.conf.Enabled {
		return false
	}
	return gameControlgameControlMgr.conf.BlockedHosts[ipStr]
}
