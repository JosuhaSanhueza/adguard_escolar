package home

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/AdguardTeam/AdGuardHome/internal/aghuser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestLabManager() (m *gameControlManager) {
	return &gameControlManager{conf: GameControlConfig{
		Enabled:      true,
		BlockedHosts: map[string]bool{},
		Labs: []GameControlLab{
			{ID: "a", Name: "A", RangeStart: "10.0.0.1", RangeEnd: "10.0.0.10"},
			{ID: "b", Name: "B", RangeStart: "10.0.1.1", RangeEnd: "10.0.1.5"},
		},
	}}
}

func TestGameControlManager_upsertLab(t *testing.T) {
	m := newTestLabManager()

	t.Run("overlap", func(t *testing.T) {
		_, err := m.upsertLab(&GameControlLab{Name: "C", RangeStart: "10.0.0.5", RangeEnd: "10.0.0.20"})
		require.Error(t, err)
	})

	t.Run("bad_range", func(t *testing.T) {
		_, err := m.upsertLab(&GameControlLab{Name: "C", RangeStart: "10.0.2.9", RangeEnd: "10.0.2.1"})
		require.Error(t, err)

		_, err = m.upsertLab(&GameControlLab{Name: "C", RangeStart: "nope", RangeEnd: "10.0.2.1"})
		require.Error(t, err)
	})

	t.Run("new", func(t *testing.T) {
		lab := &GameControlLab{Name: "C", RangeStart: "10.0.2.1", RangeEnd: "10.0.2.3"}
		_, err := m.upsertLab(lab)
		require.NoError(t, err)
		assert.NotEmpty(t, lab.ID)
		assert.Len(t, m.conf.Labs, 3)
	})

	t.Run("shrink_releases", func(t *testing.T) {
		released, err := m.upsertLab(&GameControlLab{
			ID: "b", Name: "B", RangeStart: "10.0.1.1", RangeEnd: "10.0.1.3",
		})
		require.NoError(t, err)
		assert.Equal(t, []string{"10.0.1.4", "10.0.1.5"}, released)
	})
}

func TestGameControl_scope(t *testing.T) {
	m := newTestLabManager()
	teacher := &aghuser.User{Login: "t", Role: aghuser.RoleTeacher, LabID: "b"}
	reqCtx := withWebUser(t.Context(), teacher)

	assert.True(t, m.ipAllowed(reqCtx, "10.0.1.2"))
	assert.False(t, m.ipAllowed(reqCtx, "10.0.0.2"), "other lab")
	assert.False(t, m.ipAllowed(reqCtx, "8.8.8.8"), "outside every lab")

	labs, ok := m.scopedLabsByID(reqCtx, "a")
	assert.False(t, ok, "teacher asking for another lab")
	assert.Empty(t, labs)

	labs, ok = m.scopedLabsByID(reqCtx, "")
	assert.True(t, ok)
	assert.Len(t, labs, 1)

	admin := withWebUser(t.Context(), &aghuser.User{Login: "admin"})
	assert.True(t, m.ipAllowed(admin, "10.0.0.2"))
	labs, _ = m.scopedLabsByID(admin, "")
	assert.Len(t, labs, 2)
}

func TestAuth_teachers(t *testing.T) {
	ctx := t.Context()
	db := aghuser.NewDefaultDB()
	require.NoError(t, db.Create(ctx, (&webUser{Name: "admin", PasswordHash: "x"}).toUser()))
	a := &auth{users: db}

	require.NoError(t, a.upsertTeacher(ctx, "prof1", "secret1", "a"))
	require.Error(t, a.upsertTeacher(ctx, "prof2", "", "b"), "new teacher needs a password")
	require.Error(t, a.upsertTeacher(ctx, "admin", "secret1", "a"), "must not overwrite admins")

	u, _ := db.ByLogin(ctx, "prof1")
	require.True(t, u.IsTeacher())
	assert.True(t, u.Password.Authenticate(ctx, "secret1"))

	// Update the lab only: the password and ID are kept.
	id := u.ID
	require.NoError(t, a.upsertTeacher(ctx, "prof1", "", "b"))
	u, _ = db.ByLogin(ctx, "prof1")
	assert.Equal(t, "b", u.LabID)
	assert.Equal(t, id, u.ID)
	assert.True(t, u.Password.Authenticate(ctx, "secret1"))

	assert.False(t, a.deleteTeacher(ctx, "admin"), "must not delete admins")
	assert.True(t, a.deleteTeacher(ctx, "prof1"))
	assert.Len(t, a.teachers(ctx), 0)
}

func TestRestrictTeachers(t *testing.T) {
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusOK) })
	h := restrictTeachers(next)

	do := func(u *aghuser.User, method, path string) int {
		r := httptest.NewRequest(method, path, nil)
		if u != nil {
			r = r.WithContext(withWebUser(r.Context(), u))
		}
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)

		return w.Code
	}

	teacher := &aghuser.User{Login: "t", Role: aghuser.RoleTeacher, LabID: "a"}
	admin := &aghuser.User{Login: "admin"}

	assert.Equal(t, 200, do(teacher, http.MethodGet, "/control/gamecontrol/status"))
	assert.Equal(t, 200, do(teacher, http.MethodPost, "/control/gamecontrol/toggle_all"))
	assert.Equal(t, 200, do(teacher, http.MethodGet, "/"))
	assert.Equal(t, 403, do(teacher, http.MethodGet, "/control/querylog"))
	assert.Equal(t, 403, do(teacher, http.MethodPost, "/control/access/set"))
	assert.Equal(t, 403, do(teacher, http.MethodPost, "/control/gamecontrol/labs/save"))
	assert.Equal(t, 403, do(teacher, http.MethodGet, "/control/config_profile/export"))
	assert.Equal(t, 200, do(admin, http.MethodPost, "/control/access/set"))
	assert.Equal(t, 200, do(nil, http.MethodGet, "/control/querylog"))
}

func TestWebAPI_handleLabDelete_withTeacher(t *testing.T) {
	ctx := t.Context()
	db := aghuser.NewDefaultDB()
	a := &auth{users: db}
	require.NoError(t, a.upsertTeacher(ctx, "prof", "secret1", "lab-x"))

	web := &webAPI{auth: a}
	body := strings.NewReader(`{"id":"lab-x"}`)
	r := httptest.NewRequest(http.MethodPost, "/control/gamecontrol/labs/delete", body)
	w := httptest.NewRecorder()

	// Must answer 409 and not panic on error logging.
	web.handleLabDelete(w, r)
	assert.Equal(t, http.StatusConflict, w.Code)
}
