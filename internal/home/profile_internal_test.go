package home

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	yaml "go.yaml.in/yaml/v4"
)

func TestExportImportProfile(t *testing.T) {
	sourceYAML := `
http:
  address: 192.168.1.10:3000
users:
  - name: admin
    password: sourcehash
language: es
theme: auto
dns:
  bind_hosts:
    - 0.0.0.0
  port: 53
  upstream_dns:
    - tls://1.1.1.1
filters:
  - enabled: true
    url: https://example.com/list1.txt
    name: List1
    id: 1
filtering:
  safebrowsing_enabled: true
querylog:
  enabled: true
  interval: 168h
  dir_path: /var/lib/source/data
statistics:
  enabled: true
  interval: 24h
  dir_path: /var/lib/source/data
`

	targetYAML := `
http:
  address: 192.168.2.50:3000
users:
  - name: admin
    password: targethash
language: en
theme: dark
dns:
  bind_hosts:
    - 0.0.0.0
  port: 53
  upstream_dns:
    - 8.8.8.8
filters: []
filtering:
  safebrowsing_enabled: false
querylog:
  enabled: true
  interval: 24h
  dir_path: /var/lib/target/data
statistics:
  enabled: true
  interval: 1h
  dir_path: /var/lib/target/data
`

	dir := t.TempDir()
	sourcePath := filepath.Join(dir, "source.yaml")
	targetPath := filepath.Join(dir, "target.yaml")
	profilePath := filepath.Join(dir, "profile.yaml")

	require.NoError(t, os.WriteFile(sourcePath, []byte(sourceYAML), 0o644))
	require.NoError(t, os.WriteFile(targetPath, []byte(targetYAML), 0o644))

	err := exportProfile(sourcePath, profilePath)
	require.NoError(t, err)

	profileData, err := os.ReadFile(profilePath)
	require.NoError(t, err)

	profile := yamlMap{}
	require.NoError(t, yaml.Unmarshal(profileData, &profile))

	// Installation-specific top-level keys must never be exported.
	for _, k := range []string{"http", "users"} {
		_, ok := profile[k]
		assert.Falsef(t, ok, "profile must not contain %q", k)
	}

	// Host-specific nested keys must be stripped even though their parent
	// section is portable.
	dns, ok := profile["dns"].(yamlMap)
	require.True(t, ok)
	assert.NotContains(t, dns, "bind_hosts")
	assert.NotContains(t, dns, "port")

	querylog, ok := profile["querylog"].(yamlMap)
	require.True(t, ok)
	assert.NotContains(t, querylog, "dir_path")

	err = importProfile(targetPath, profilePath)
	require.NoError(t, err)

	targetData, err := os.ReadFile(targetPath)
	require.NoError(t, err)

	merged := yamlMap{}
	require.NoError(t, yaml.Unmarshal(targetData, &merged))

	// Installation-specific settings are preserved from the target.
	httpCfg, ok := merged["http"].(yamlMap)
	require.True(t, ok)
	assert.Equal(t, "192.168.2.50:3000", httpCfg["address"])

	users, ok := merged["users"].([]any)
	require.True(t, ok)
	require.Len(t, users, 1)
	user, ok := users[0].(yamlMap)
	require.True(t, ok)
	assert.Equal(t, "targethash", user["password"])

	mergedQuerylog, ok := merged["querylog"].(yamlMap)
	require.True(t, ok)
	assert.Equal(t, "/var/lib/target/data", mergedQuerylog["dir_path"])

	// Portable settings are overwritten with the profile's values.
	assert.Equal(t, "es", merged["language"])
	assert.Equal(t, "auto", merged["theme"])
	assert.Equal(t, "168h", mergedQuerylog["interval"])

	mergedDNS, ok := merged["dns"].(yamlMap)
	require.True(t, ok)
	assert.Equal(t, []any{"tls://1.1.1.1"}, mergedDNS["upstream_dns"])

	mergedFilters, ok := merged["filters"].([]any)
	require.True(t, ok)
	require.Len(t, mergedFilters, 1)

	mergedFiltering, ok := merged["filtering"].(yamlMap)
	require.True(t, ok)
	assert.Equal(t, true, mergedFiltering["safebrowsing_enabled"])
}
