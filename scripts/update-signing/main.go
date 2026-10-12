// Command update-signing generates the EscolarApp Shield update signing key and
// signs release packages with it.
//
// Usage:
//
//	update-signing genkey PRIVATE_KEY_FILE
//	UPDATE_SIGNING_KEY=... update-signing sign PACKAGE...
//
// genkey writes a new private key to PRIVATE_KEY_FILE (mode 0600) and prints
// the matching public key, which goes into internal/updater/signature.go.  sign
// writes PACKAGE.sig next to every PACKAGE using the private key from the
// UPDATE_SIGNING_KEY environment variable, and checks the result.
package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os"
	"strings"

	"github.com/AdguardTeam/AdGuardHome/internal/updater"
)

func main() {
	if len(os.Args) < 3 {
		fail("usage: update-signing genkey PRIVATE_KEY_FILE | update-signing sign PACKAGE...")
	}

	switch os.Args[1] {
	case "genkey":
		genKey(os.Args[2])
	case "sign":
		sign(os.Args[2:])
	default:
		fail("unknown command %q", os.Args[1])
	}
}

func fail(format string, args ...any) {
	_, _ = fmt.Fprintf(os.Stderr, "update-signing: "+format+"\n", args...)
	os.Exit(1)
}

func genKey(path string) {
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		fail("generating key: %s", err)
	}

	seed := base64.StdEncoding.EncodeToString(priv.Seed())
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		fail("creating key file: %s", err)
	}

	_, err = fmt.Fprintln(f, seed)
	if err != nil {
		fail("writing key file: %s", err)
	}

	err = f.Close()
	if err != nil {
		fail("closing key file: %s", err)
	}

	fmt.Printf("private key written to %s\n", path)
	fmt.Printf("public key: %s\n", base64.StdEncoding.EncodeToString(pub))
}

func sign(pkgs []string) {
	seed, err := base64.StdEncoding.DecodeString(strings.TrimSpace(os.Getenv("UPDATE_SIGNING_KEY")))
	if err != nil || len(seed) != ed25519.SeedSize {
		fail("UPDATE_SIGNING_KEY must be the base64 private key written by genkey")
	}

	priv := ed25519.NewKeyFromSeed(seed)
	for _, p := range pkgs {
		data, rerr := os.ReadFile(p)
		if rerr != nil {
			fail("reading %s: %s", p, rerr)
		}

		sig := base64.StdEncoding.EncodeToString(ed25519.Sign(priv, updater.SignedMessage(data)))
		werr := os.WriteFile(p+updater.SignatureSuffix, []byte(sig+"\n"), 0o644)
		if werr != nil {
			fail("writing signature: %s", werr)
		}

		// Make sure the binaries built from this tree accept the signature.
		verr := updater.VerifyPackage(updater.DefaultSigningKey(), data, []byte(sig))
		if verr != nil {
			fail("%s: signed with a key that doesn't match internal/updater: %s", p, verr)
		}

		fmt.Printf("signed %s\n", p)
	}

}
