package home

import (
	"encoding/json"
	"net/http"

	"github.com/AdguardTeam/AdGuardHome/internal/aghhttp"
	"github.com/AdguardTeam/golibs/httphdr"
	"github.com/google/renameio/v2/maybe"
)

// maxConfigProfileSize is the maximum accepted size, in bytes, of an
// uploaded configuration profile.  A real profile is a few KiB at most;
// this is a generous bound against accidental or malicious uploads.
const maxConfigProfileSize = 1024 * 1024

// handleConfigProfileExport is the handler for GET
// /control/config_profile/export.  It responds with the portable subset of
// the current configuration (DNS behavior, blocklists, blocked services,
// safe search, cache, query log and statistics retention) as a downloadable
// YAML file, for reuse on another AdGuard Home installation.
func (web *webAPI) handleConfigProfileExport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := web.logger

	profileYAML, err := buildProfileYAML(web.conf.confPath)
	if err != nil {
		aghhttp.ErrorAndLog(ctx, l, r, w, http.StatusInternalServerError, "building profile: %s", err)

		return
	}

	w.Header().Set(httphdr.ContentType, "application/yaml")
	w.Header().Set(httphdr.ContentDisposition, `attachment; filename="adguard-escolar-profile.yaml"`)

	_, _ = w.Write(profileYAML)
}

// configProfileImportReq is the request body for
// POST /control/config_profile/import.
type configProfileImportReq struct {
	// Profile is the full text of a profile file previously produced by
	// handleConfigProfileExport (or --export-profile).
	Profile string `json:"profile"`
}

// handleConfigProfileImport is the handler for POST
// /control/config_profile/import.  It merges the uploaded profile into the
// current configuration file and responds with 200 OK.  The running
// instance must be restarted for the imported settings to take effect,
// since the in-memory configuration isn't touched.
func (web *webAPI) handleConfigProfileImport(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	l := web.logger

	r.Body = http.MaxBytesReader(w, r.Body, maxConfigProfileSize)

	req := &configProfileImportReq{}
	if err := json.NewDecoder(r.Body).Decode(req); err != nil {
		aghhttp.ErrorAndLog(ctx, l, r, w, http.StatusBadRequest, "reading req: %s", err)

		return
	}

	if req.Profile == "" {
		aghhttp.ErrorAndLog(ctx, l, r, w, http.StatusBadRequest, "profile must not be empty")

		return
	}

	confPath := web.conf.confPath

	mergedYAML, err := mergeProfileYAML(confPath, []byte(req.Profile))
	if err != nil {
		aghhttp.ErrorAndLog(ctx, l, r, w, http.StatusBadRequest, "merging profile: %s", err)

		return
	}

	if err = maybe.WriteFile(confPath, mergedYAML, 0o644); err != nil {
		aghhttp.ErrorAndLog(ctx, l, r, w, http.StatusInternalServerError, "writing config file: %s", err)

		return
	}

	aghhttp.OK(ctx, l, w)
}
