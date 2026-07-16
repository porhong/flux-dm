package secrets

import (
	"bytes"
	"testing"
)

func TestDPAPIRoundTripAndCiphertextIsOpaque(t *testing.T) {
	plaintext := []byte("not-for-sqlite")
	ciphertext, err := (DPAPI{}).Protect(plaintext)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, plaintext) {
		t.Fatal("ciphertext contains plaintext")
	}
	actual, err := (DPAPI{}).Unprotect(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(actual, plaintext) {
		t.Fatalf("round trip=%q", actual)
	}
}
