package home

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWeb_HandleConfigProfileExportImport(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "AdGuardHome.yaml")

	require.NoError(t, os.WriteFile(confPath, []byte(`
http:
  address: 192.168.2.50:3000
language: en
dns:
  bind_hosts:
    - 0.0.0.0
  port: 53
  upstream_dns:
    - 8.8.8.8
`), 0o644))

	web := newTestWeb(t, &webConfig{confPath: confPath})

	// Export.
	exportReq := httptest.NewRequest(http.MethodGet, "/control/config_profile/export", nil)
	exportRec := httptest.NewRecorder()

	web.handleConfigProfileExport(exportRec, exportReq)

	require.Equal(t, http.StatusOK, exportRec.Code)
	assert.Contains(t, exportRec.Header().Get("Content-Disposition"), "attachment")

	exported := exportRec.Body.Bytes()
	assert.NotContains(t, string(exported), "192.168.2.50")
	assert.NotContains(t, string(exported), "bind_hosts")

	// Modify the exported profile to simulate a profile coming from a
	// different installation, then import it back.
	profileYAML := bytes.Replace(exported, []byte("language: en"), []byte("language: es"), 1)

	reqBody, err := json.Marshal(configProfileImportReq{Profile: string(profileYAML)})
	require.NoError(t, err)

	importReq := httptest.NewRequest(http.MethodPost, "/control/config_profile/import", bytes.NewReader(reqBody))
	importReq.Header.Set("Content-Type", "application/json")
	importRec := httptest.NewRecorder()

	web.handleConfigProfileImport(importRec, importReq)

	require.Equal(t, http.StatusOK, importRec.Code)

	merged, err := os.ReadFile(confPath)
	require.NoError(t, err)

	mergedStr := string(merged)
	assert.Contains(t, mergedStr, "language: es")
	// Installation-specific settings survive the import untouched.
	assert.Contains(t, mergedStr, "192.168.2.50:3000")
	assert.Contains(t, mergedStr, "bind_hosts")
}

func TestWeb_HandleConfigProfileImport_EmptyProfile(t *testing.T) {
	dir := t.TempDir()
	confPath := filepath.Join(dir, "AdGuardHome.yaml")
	require.NoError(t, os.WriteFile(confPath, []byte("language: en\n"), 0o644))

	web := newTestWeb(t, &webConfig{confPath: confPath})

	reqBody, err := json.Marshal(configProfileImportReq{Profile: ""})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/control/config_profile/import", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	web.handleConfigProfileImport(rec, req)

	assert.Equal(t, http.StatusBadRequest, rec.Code)
}
