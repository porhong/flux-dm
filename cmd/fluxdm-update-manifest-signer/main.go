// fluxdm-update-manifest-signer signs an already-written update manifest. The
// private key is read from an environment variable so it is never committed or
// passed as a command-line argument.
package main

import (
	"crypto/ed25519"
	"encoding/base64"
	"errors"
	"flag"
	"os"
	"strings"
)

func main() {
	input := flag.String("input", "", "manifest path")
	output := flag.String("output", "", "signature path")
	flag.Parse()
	if *input == "" || *output == "" {
		fail(errors.New("input and output are required"))
	}
	encoded := strings.TrimSpace(os.Getenv("FLUXDM_UPDATE_MANIFEST_PRIVATE_KEY"))
	private, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		fail(errors.New("invalid update signing key"))
	}
	if len(private) == ed25519.SeedSize {
		private = ed25519.NewKeyFromSeed(private)
	}
	if len(private) != ed25519.PrivateKeySize {
		fail(errors.New("invalid update signing key"))
	}
	payload, err := os.ReadFile(*input)
	if err != nil {
		fail(err)
	}
	signature := base64.StdEncoding.EncodeToString(ed25519.Sign(ed25519.PrivateKey(private), payload))
	if err := os.WriteFile(*output, []byte(signature+"\n"), 0o600); err != nil {
		fail(err)
	}
}
func fail(err error) {
	_, _ = os.Stderr.WriteString("update manifest signer: " + err.Error() + "\n")
	os.Exit(2)
}
