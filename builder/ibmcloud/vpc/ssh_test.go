package vpc

import (
	"crypto/ed25519"
	"crypto/rand"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"golang.org/x/crypto/ssh"
)

func newTestHostKey(t *testing.T) ssh.PublicKey {
	t.Helper()
	pub, _, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("generating key: %v", err)
	}
	k, err := ssh.NewPublicKey(pub)
	if err != nil {
		t.Fatalf("wrapping key: %v", err)
	}
	return k
}

func TestPinnedHostKeyCallback(t *testing.T) {
	state := new(multistep.BasicStateBag)
	cb := pinnedHostKeyCallback(state)
	k1, k2 := newTestHostKey(t), newTestHostKey(t)

	if err := cb("host-a:22", nil, k1); err != nil {
		t.Fatalf("first use should pin and succeed: %v", err)
	}
	if err := cb("host-a:22", nil, k1); err != nil {
		t.Fatalf("same key should succeed: %v", err)
	}
	err := cb("host-a:22", nil, k2)
	if err == nil {
		t.Fatal("expected mismatch error for a different key")
	}
	if !strings.Contains(err.Error(), ssh.FingerprintSHA256(k1)) || !strings.Contains(err.Error(), ssh.FingerprintSHA256(k2)) {
		t.Fatalf("mismatch error should include both fingerprints, got: %v", err)
	}
}

func TestPinnedHostKeyCallbackIsPerHost(t *testing.T) {
	state := new(multistep.BasicStateBag)
	cb := pinnedHostKeyCallback(state)
	k1, k2 := newTestHostKey(t), newTestHostKey(t)

	if err := cb("host-a:22", nil, k1); err != nil {
		t.Fatalf("host-a first pin: %v", err)
	}
	if err := cb("host-b:22", nil, k2); err != nil {
		t.Fatalf("host-b must get its own independent pin: %v", err)
	}
}
