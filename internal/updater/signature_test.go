package updater_test

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/AdguardTeam/AdGuardHome/internal/updater"
	"github.com/AdguardTeam/AdGuardHome/internal/version"
	"github.com/AdguardTeam/golibs/testutil"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newTestSignature returns a deterministic test key and the base64 signature
// of pkg made with it.
func newTestSignature(tb testing.TB, pkg []byte) (pub ed25519.PublicKey, sig []byte) {
	tb.Helper()

	priv := ed25519.NewKeyFromSeed(make([]byte, ed25519.SeedSize))
	pub, _ = priv.Public().(ed25519.PublicKey)
	raw := ed25519.Sign(priv, updater.SignedMessage(pkg))

	return pub, []byte(base64.StdEncoding.EncodeToString(raw) + "\n")
}

func TestDefaultSigningKey(t *testing.T) {
	assert.Len(t, updater.DefaultSigningKey(), ed25519.PublicKeySize)
}

func TestVerifyPackage(t *testing.T) {
	pkg := []byte("package data")
	key, sig := newTestSignature(t, pkg)

	assert.NoError(t, updater.VerifyPackage(key, pkg, sig))
	assert.Error(t, updater.VerifyPackage(key, []byte("tampered data"), sig))
	assert.Error(t, updater.VerifyPackage(key, pkg, []byte("not base64!")))
	assert.Error(t, updater.VerifyPackage(updater.DefaultSigningKey(), pkg, sig), "wrong key")
}

func TestUpdater_Update_badSignature(t *testing.T) {
	const packagePath = "/AdGuardHome.tar.gz"

	pkgData, readErr := os.ReadFile("testdata/AdGuardHome_unix.tar.gz")
	require.NoError(t, readErr)

	testKey, goodSig := newTestSignature(t, pkgData)
	_, otherSig := newTestSignature(t, []byte("another package"))

	testCases := []struct {
		name string
		sig  []byte
		code int
	}{{
		sig:  goodSig,
		name: "good",
		code: http.StatusOK,
	}, {
		sig:  otherSig,
		name: "mismatch",
		code: http.StatusOK,
	}, {
		sig:  nil,
		name: "missing",
		code: http.StatusNotFound,
	}}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			wd := t.TempDir()
			exePath := filepath.Join(wd, "AdGuardHome")
			yamlPath := filepath.Join(wd, "AdGuardHome.yaml")
			require.NoError(t, os.WriteFile(exePath, []byte("old binary"), 0o755))

			mux := http.NewServeMux()
			mux.HandleFunc(packagePath, func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write(pkgData)
			})
			mux.HandleFunc(packagePath+updater.SignatureSuffix, func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tc.code)
				_, _ = w.Write(tc.sig)
			})

			versionPath := path.Join("/adguardhome", version.ChannelBeta, "version.json")
			mux.HandleFunc(versionPath, func(w http.ResponseWriter, r *http.Request) {
				pkgURL, _ := url.JoinPath("http://", r.Host, packagePath)
				_, _ = fmt.Fprintf(w, `{"version": "v0.103.0-beta.2", "announcement": "test", `+
					`"announcement_url": "https://example.com", "download_linux_amd64": %q}`, pkgURL)
			})

			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)
			srvURL, err := url.Parse(srv.URL)
			require.NoError(t, err)

			u := updater.NewUpdater(&updater.Config{
				Client:             srv.Client(),
				Logger:             testLogger,
				CommandConstructor: testCmdCons,
				GOARCH:             "amd64",
				GOOS:               "linux",
				Version:            "v0.103.0",
				ConfName:           yamlPath,
				WorkDir:            wd,
				ExecPath:           exePath,
				VersionCheckURL:    srvURL.JoinPath(versionPath),
				SigningKey:         testKey,
			})

			_, err = u.VersionInfo(testutil.ContextWithTimeout(t, testTimeout), false)
			require.NoError(t, err)

			err = u.Update(testutil.ContextWithTimeout(t, testTimeout), true)
			if tc.name == "good" {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			// The installed binary must be left untouched.
			data, rerr := os.ReadFile(exePath)
			require.NoError(t, rerr)
			assert.Equal(t, "old binary", string(data))
		})
	}
}
