package home

import (
	"log/slog"
	"net/http"
	"strings"

	"github.com/AdguardTeam/AdGuardHome/internal/aghhttp"
)

// teacherAllowed is the set of "METHOD path" API requests a teacher may make.
var teacherAllowed = map[string]bool{
	"GET /control/profile":                           true,
	"GET /control/status":                            true,
	"GET /control/logout":                            true,
	"GET /control/gamecontrol/status":                true,
	"POST /control/gamecontrol/update_host":          true,
	"POST /control/gamecontrol/toggle_all":           true,
	"POST /control/gamecontrol/internet/toggle_host": true,
	"POST /control/gamecontrol/internet/toggle_all":  true,
}

// restrictTeachers rejects every API request of a teacher user that is not in
// [teacherAllowed].  Non-API paths (the static frontend) are not restricted.
// It must run after authentication has put the user into the context.
func restrictTeachers(next http.Handler) (h http.Handler) {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		u, ok := webUserFromContext(ctx)
		if ok && u.IsTeacher() && strings.HasPrefix(r.URL.Path, "/control/") &&
			!teacherAllowed[r.Method+" "+r.URL.Path] {
			aghhttp.ErrorAndLog(ctx, slog.Default(), r, w, http.StatusForbidden, "forbidden for this account")

			return
		}

		next.ServeHTTP(w, r)
	})
}
