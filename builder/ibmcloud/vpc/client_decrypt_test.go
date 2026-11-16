package vpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

func newKeyPairState(t *testing.T) *multistep.BasicStateBag {
	t.Helper()
	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("config", Config{})
	step := &stepCreateSshKeyPair{}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("stepCreateSshKeyPair.Run returned %v", action)
	}
	return state
}

func TestDecryptPassword_UsesInMemoryKey(t *testing.T) {
	state := newKeyPairState(t)

	block, _ := pem.Decode(state.Get("ssh_private_key_pem").([]byte))
	if block == nil {
		t.Fatal("ssh_private_key_pem is not valid PEM")
	}
	rsaKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}

	want := "S3cret-P@ssw0rd"
	encrypted, err := rsa.EncryptPKCS1v15(rand.Reader, &rsaKey.PublicKey, []byte(want))
	if err != nil {
		t.Fatal(err)
	}

	got, err := IBMCloudClient{}.DecryptPassword(encrypted, state)
	if err != nil {
		t.Fatalf("DecryptPassword: %v", err)
	}
	if got != want {
		t.Errorf("DecryptPassword = %q, want %q", got, want)
	}
}

func TestStepCreateSshKeyPair_WritesNothingToDisk(t *testing.T) {
	dir := t.TempDir()
	t.Chdir(dir)

	state := newKeyPairState(t)
	(&stepCreateSshKeyPair{}).Cleanup(state)

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty working directory, found %d entries", len(entries))
	}
}

func TestDecryptPassword_MissingKey(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))

	if _, err := (IBMCloudClient{}).DecryptPassword([]byte("x"), state); err == nil {
		t.Fatal("expected error when ssh_private_key_pem is absent")
	}
}

func TestDecryptPassword_NonRSAKey(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("config", Config{SshKeyType: "ed25519"})
	step := &stepCreateSshKeyPair{}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("stepCreateSshKeyPair.Run returned %v", action)
	}

	if _, err := (IBMCloudClient{}).DecryptPassword([]byte("x"), state); err == nil {
		t.Fatal("expected error for non-RSA key")
	}
}
