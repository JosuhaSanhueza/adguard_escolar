package updater

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/AdguardTeam/golibs/errors"
	"github.com/AdguardTeam/golibs/ioutil"
)

// SignatureSuffix is appended to a package URL to get the URL of its detached
// signature.
const SignatureSuffix = ".sig"

// maxSignatureFileSize is the maximum size of a signature file.  A base64
// ed25519 signature is 88 bytes.
const maxSignatureFileSize = 1024

// signedMessagePrefix makes signatures of update packages useless for anything
// else.
const signedMessagePrefix = "escolarapp-shield-update-v1\n"

// defaultSigningKey is the base64-encoded ed25519 public key that update
// packages must be signed with.  The private key is never stored on the update
// server, so a compromised server can't push its own binaries.
const defaultSigningKey = "aadffatSWma6HEJPN3uVcpKDIxFhnltcZGaQM2jWmSM="

// DefaultSigningKey returns the public key that update packages must be signed
// with.
func DefaultSigningKey() (key ed25519.PublicKey) {
	key, err := ParseSigningKey(defaultSigningKey)
	if err != nil {
		panic(fmt.Errorf("bad built-in signing key: %w", err))
	}

	return key
}

// ParseSigningKey parses a base64-encoded ed25519 public key.
func ParseSigningKey(s string) (key ed25519.PublicKey, err error) {
	b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("decoding key: %w", err)
	}

	if len(b) != ed25519.PublicKeySize {
		return nil, fmt.Errorf("key size: got %d, want %d", len(b), ed25519.PublicKeySize)
	}

	return ed25519.PublicKey(b), nil
}

// SignedMessage returns the message that is signed for the package data pkg.
func SignedMessage(pkg []byte) (msg []byte) {
	sum := sha256.Sum256(pkg)

	return []byte(signedMessagePrefix + hex.EncodeToString(sum[:]) + "\n")
}

// VerifyPackage returns an error if sig, a base64-encoded signature, is not a
// valid signature of the package data pkg made with key.
func VerifyPackage(key ed25519.PublicKey, pkg, sig []byte) (err error) {
	rawSig, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(sig)))
	if err != nil {
		return fmt.Errorf("decoding signature: %w", err)
	}

	if !ed25519.Verify(key, SignedMessage(pkg), rawSig) {
		return errors.Error("signature does not match the package")
	}

	return nil
}

// verifyPackageFile downloads the detached signature of the package and checks
// the downloaded package file against it.
func (u *Updater) verifyPackageFile(ctx context.Context) (err error) {
	sig, err := u.downloadSignature(ctx)
	if err != nil {
		return fmt.Errorf("downloading signature: %w", err)
	}

	pkg, err := os.ReadFile(u.packageName)
	if err != nil {
		return fmt.Errorf("reading package file: %w", err)
	}

	err = VerifyPackage(u.signingKey, pkg, sig)
	if err != nil {
		return fmt.Errorf("verifying %s: %w", u.packageURL, err)
	}

	u.logger.InfoContext(ctx, "package signature verified")

	return nil
}

// downloadSignature returns the contents of the package's signature file.
func (u *Updater) downloadSignature(ctx context.Context) (sig []byte, err error) {
	sigURL := u.packageURL + SignatureSuffix
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sigURL, nil)
	if err != nil {
		return nil, fmt.Errorf("constructing request: %w", err)
	}

	resp, err := u.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("requesting %s: %w", sigURL, err)
	}
	defer func() { err = errors.WithDeferred(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("requesting %s: status %d", sigURL, resp.StatusCode)
	}

	sig, err = io.ReadAll(ioutil.LimitReader(resp.Body, maxSignatureFileSize))
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", sigURL, err)
	}

	return sig, nil
}
