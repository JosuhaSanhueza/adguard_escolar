package home

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"github.com/AdguardTeam/AdGuardHome/internal/aghhttp"
	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/httphdr"
	"github.com/AdguardTeam/golibs/logutil/slogutil"
	"github.com/AdguardTeam/golibs/osutil/executil"
	"github.com/google/renameio/v2/maybe"
)

// profileFileName is the name offered for downloaded configuration profiles.
const profileFileName = "escolarapp-shield-perfil.yaml"

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
	w.Header().Set(httphdr.ContentDisposition, `attachment; filename="`+profileFileName+`"`)

	_, _ = w.Write(profileYAML)
}

// configProfileImportReq is the request body for
// POST /control/config_profile/import.
type configProfileImportReq struct {
	// Profile is the full text of a profile file previously produced by
	// handleConfigProfileExport (or --export-profile).
	Profile string `json:"profile"`
}

// checkProfileConfig runs the current executable with --check-config against
// the configuration file at path, the same check the updater runs before
// switching to a new version.  It is a variable so tests can replace it.
var checkProfileConfig = func(ctx context.Context, web *webAPI, execPath, path string) (err error) {
	var buf bytes.Buffer
	err = executil.Run(ctx, web.cmdCons, &executil.CommandConfig{
		Path:   execPath,
		Args:   []string{"-c", path, "-w", web.conf.workDir, "--check-config"},
		Stdout: &buf,
		Stderr: &buf,
	})
	if err != nil {
		return fmt.Errorf("%w: %s", err, bytes.TrimSpace(buf.Bytes()))
	}

	return nil
}

// restartAfterProfileImport restarts the process so that the imported
// configuration takes effect, the same way it restarts after an update.  It is
// a variable so tests can replace it.
var restartAfterProfileImport = func(web *webAPI, execPath string) {
	// The background context is used because the restart outlives the
	// request.
	go web.finishUpdate(context.Background(), execPath)
}

// handleConfigProfileImport is the handler for POST
// /control/config_profile/import.  It merges the uploaded profile into the
// current configuration file, checks the result the same way the updater does,
// and restarts the process so that the imported settings take effect at once.
// Restarting right away also means no later configuration write from the
// running instance can overwrite the imported file.
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

	execPath, err := os.Executable()
	if err != nil {
		aghhttp.ErrorAndLog(ctx, l, r, w, http.StatusInternalServerError, "getting path: %s", err)

		return
	}

	err = web.applyProfile(ctx, execPath, []byte(req.Profile))
	if err != nil {
		aghhttp.ErrorAndLog(ctx, l, r, w, http.StatusBadRequest, "%s", err)

		return
	}

	aghhttp.OK(ctx, l, w)

	err = http.NewResponseController(w).Flush()
	if err != nil {
		l.WarnContext(ctx, "flushing response", slogutil.KeyError, err)
	}

	l.InfoContext(ctx, "configuration profile imported, restarting")
	restartAfterProfileImport(web, execPath)
}

// applyProfile merges profileYAML into the configuration file, after checking
// that the result is a configuration the executable at execPath accepts.  The
// configuration file is left untouched on error.
func (web *webAPI) applyProfile(ctx context.Context, execPath string, profileYAML []byte) (err error) {
	confPath := web.conf.confPath

	mergedYAML, err := mergeProfileYAML(confPath, profileYAML)
	if err != nil {
		return fmt.Errorf("merging profile: %w", err)
	}

	checkPath := confPath + ".import-check.yaml"
	err = os.WriteFile(checkPath, mergedYAML, 0o600)
	if err != nil {
		return fmt.Errorf("writing configuration to check: %w", err)
	}
	defer func() { _ = os.Remove(checkPath) }()

	err = checkProfileConfig(ctx, web, execPath, checkPath)
	if err != nil {
		// The check output is meant for the logs, not for the user.
		web.logger.WarnContext(ctx, "rejected configuration profile", slogutil.KeyError, err)

		return errors.Error(
			"el archivo no es un perfil válido de EscolarApp Shield; no se aplicó ningún cambio",
		)
	}

	err = maybe.WriteFile(confPath, mergedYAML, 0o644)
	if err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}
