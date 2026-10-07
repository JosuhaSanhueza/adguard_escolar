package home

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"strings"

	"github.com/AdguardTeam/AdGuardHome/internal/aghhttp"
	"github.com/AdguardTeam/AdGuardHome/internal/aghuser"
)

const minTeacherPasswordLen = 6

func (web *webAPI) registerLabHandlers() {
	reg := web.httpReg

	reg.Register(http.MethodPost, "/control/gamecontrol/labs/save", web.handleLabSave)
	reg.Register(http.MethodPost, "/control/gamecontrol/labs/delete", web.handleLabDelete)
	reg.Register(http.MethodGet, "/control/gamecontrol/teachers", web.handleTeachersList)
	reg.Register(http.MethodPost, "/control/gamecontrol/teachers/save", web.handleTeacherSave)
	reg.Register(http.MethodPost, "/control/gamecontrol/teachers/delete", web.handleTeacherDelete)
}

// requireAdmin writes a 403 response and returns false if the request's user
// is a restricted teacher.
func requireAdmin(w http.ResponseWriter, r *http.Request) (ok bool) {
	if _, restricted := labScope(r.Context()); restricted {
		aghhttp.ErrorAndLog(r.Context(), slog.Default(), r, w, http.StatusForbidden, "administrators only")

		return false
	}

	return true
}

func newLabID() (id string) {
	b := make([]byte, 6)
	_, _ = rand.Read(b)

	return "lab-" + hex.EncodeToString(b)
}

// validateLabRange checks the range of lab and makes sure it doesn't overlap
// with any other lab.  It must be called with m.mu held.
func (m *gameControlManager) validateLab(lab GameControlLab) (err error) {
	if lab.Name == "" {
		return fmt.Errorf("el nombre del laboratorio no puede estar vacío")
	}

	start, ok1 := parseIP4(lab.RangeStart)
	end, ok2 := parseIP4(lab.RangeEnd)
	switch {
	case !ok1 || !ok2:
		return fmt.Errorf("las IPs de inicio y fin deben ser direcciones IPv4 válidas")
	case start > end:
		return fmt.Errorf("la IP de inicio debe ser menor o igual a la IP de fin")
	case end-start >= maxLabHosts:
		return fmt.Errorf("el rango no puede superar %d equipos", maxLabHosts)
	}

	for _, other := range m.conf.Labs {
		if other.ID == lab.ID {
			continue
		}

		os, _ := parseIP4(other.RangeStart)
		oe, _ := parseIP4(other.RangeEnd)
		if start <= oe && os <= end {
			return fmt.Errorf("el rango se superpone con el laboratorio %q", other.Name)
		}
	}

	return nil
}

type labSaveReq struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	RangeStart string `json:"range_start"`
	RangeEnd   string `json:"range_end"`
}

// handleLabSave creates a lab (empty ID) or updates an existing one.
func (web *webAPI) handleLabSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !requireAdmin(w, r) {
		return
	}

	req := &labSaveReq{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "invalid request: %s", err)

		return
	}

	lab := GameControlLab{
		ID:         req.ID,
		Name:       strings.TrimSpace(req.Name),
		RangeStart: strings.TrimSpace(req.RangeStart),
		RangeEnd:   strings.TrimSpace(req.RangeEnd),
	}

	released, err := gameControlgameControlMgr.upsertLab(&lab)
	if err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "%s", err)

		return
	}

	releaseInternet(ctx, released)
	gameControlgameControlMgr.save(ctx)

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, lab)
}

// upsertLab validates and stores lab, assigning an ID if it has none.  It
// returns the IPs that stopped belonging to any lab.
func (m *gameControlManager) upsertLab(lab *GameControlLab) (released []string, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if lab.ID == "" {
		lab.ID = newLabID()
	}

	err = m.validateLab(*lab)
	if err != nil {
		return nil, err
	}

	i := slices.IndexFunc(m.conf.Labs, func(l GameControlLab) bool { return l.ID == lab.ID })
	if i < 0 {
		m.conf.Labs = append(m.conf.Labs, *lab)

		return nil, nil
	}

	before := m.labHosts(m.conf.Labs[i])
	m.conf.Labs[i] = *lab
	after := m.labHosts(*lab)

	for _, h := range before {
		if !slices.ContainsFunc(after, func(a GameControlHost) bool { return a.IP == h.IP }) {
			released = append(released, h.IP)
			delete(m.conf.BlockedHosts, h.IP)
		}
	}

	return released, nil
}

// releaseInternet restores internet access to ips, so hosts that left every
// lab don't stay cut off with no way to restore them from the lab panel.
func releaseInternet(ctx context.Context, ips []string) {
	dnsServer := globalContext.dnsServer
	if dnsServer == nil || len(ips) == 0 {
		return
	}

	clients := dnsServer.DisallowedClients()
	kept := slices.DeleteFunc(slices.Clone(clients), func(c string) bool { return slices.Contains(ips, c) })
	if len(kept) == len(clients) {
		return
	}

	_ = dnsServer.SetDisallowedClients(ctx, kept)
}

type labDeleteReq struct {
	ID string `json:"id"`
}

// handleLabDelete removes a lab.  The lab must not have a teacher assigned.
func (web *webAPI) handleLabDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !requireAdmin(w, r) {
		return
	}

	req := &labDeleteReq{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "invalid request: %s", err)

		return
	}

	if t := web.teacherOfLab(ctx, req.ID); t != "" {
		aghhttp.ErrorAndLog(
			ctx, slog.Default(), r, w, http.StatusConflict,
			"el laboratorio tiene al docente %q asignado; elimínalo primero", t,
		)

		return
	}

	m := gameControlgameControlMgr
	m.mu.Lock()
	var released []string
	if i := slices.IndexFunc(m.conf.Labs, func(l GameControlLab) bool { return l.ID == req.ID }); i >= 0 {
		for _, h := range m.labHosts(m.conf.Labs[i]) {
			released = append(released, h.IP)
			delete(m.conf.BlockedHosts, h.IP)
		}

		m.conf.Labs = slices.Delete(m.conf.Labs, i, i+1)
	}
	m.mu.Unlock()

	releaseInternet(ctx, released)
	m.save(ctx)

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, map[string]string{"result": "ok"})
}

// teacherOfLab returns the login of the teacher assigned to labID, if any.
func (web *webAPI) teacherOfLab(ctx context.Context, labID string) (login string) {
	for _, t := range web.auth.teachers(ctx) {
		if t.LabID == labID {
			return string(t.Login)
		}
	}

	return ""
}

type teacherJSON struct {
	Login string `json:"login"`
	LabID string `json:"lab_id"`
}

func (web *webAPI) handleTeachersList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !requireAdmin(w, r) {
		return
	}

	list := []teacherJSON{}
	for _, t := range web.auth.teachers(ctx) {
		list = append(list, teacherJSON{Login: string(t.Login), LabID: t.LabID})
	}

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, list)
}

type teacherSaveReq struct {
	Login    string `json:"login"`
	Password string `json:"password"`
	LabID    string `json:"lab_id"`
}

// handleTeacherSave creates a teacher account or updates the lab and,
// optionally, the password of an existing one.
func (web *webAPI) handleTeacherSave(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !requireAdmin(w, r) {
		return
	}

	req := &teacherSaveReq{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "invalid request: %s", err)

		return
	}

	err := web.saveTeacher(ctx, req)
	if err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "%s", err)

		return
	}

	web.confModifier.Apply(ctx)

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, map[string]string{"result": "ok"})
}

func (web *webAPI) saveTeacher(ctx context.Context, req *teacherSaveReq) (err error) {
	login, err := aghuser.NewLogin(strings.TrimSpace(req.Login))
	if err != nil {
		return fmt.Errorf("usuario inválido: %w", err)
	}

	m := gameControlgameControlMgr
	m.mu.RLock()
	labExists := slices.ContainsFunc(m.conf.Labs, func(l GameControlLab) bool { return l.ID == req.LabID })
	m.mu.RUnlock()
	if !labExists {
		return fmt.Errorf("el laboratorio no existe")
	}

	if other := web.teacherOfLab(ctx, req.LabID); other != "" && other != string(login) {
		return fmt.Errorf("el laboratorio ya tiene al docente %q asignado", other)
	}

	if req.Password != "" && len(req.Password) < minTeacherPasswordLen {
		return fmt.Errorf("la contraseña debe tener al menos %d caracteres", minTeacherPasswordLen)
	}

	return web.auth.upsertTeacher(ctx, login, req.Password, req.LabID)
}

type teacherDeleteReq struct {
	Login string `json:"login"`
}

func (web *webAPI) handleTeacherDelete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	if !requireAdmin(w, r) {
		return
	}

	req := &teacherDeleteReq{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusBadRequest, "invalid request: %s", err)

		return
	}

	if !web.auth.deleteTeacher(ctx, aghuser.Login(req.Login)) {
		aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusNotFound, "docente no encontrado")

		return
	}

	web.confModifier.Apply(ctx)

	aghhttp.WriteJSONResponseOK(ctx, slog.Default(), w, r, map[string]string{"result": "ok"})
}
