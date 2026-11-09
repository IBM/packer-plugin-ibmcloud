package vpc

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// sshConfig used to call log.Fatalf on a bad key, which killed the plugin
// process and skipped every step's Cleanup. These tests prove it now returns an
// error instead (a log.Fatalf would abort the test binary).
//
// communicator.StepConnect calls sshConfig/winRMConfig on every retry and only
// logs the returned error, so these callbacks must NOT record "error" in state:
// a stale error would fail an otherwise successful build (see buildResultError).
func TestSSHConfigMissingKeyReturnsError(t *testing.T) {
	state := new(multistep.BasicStateBag)
	state.Put("config", Config{})
	state.Put("ui", packer.TestUi(t))
	state.Put("PRIVATE_KEY", filepath.Join(t.TempDir(), "does-not-exist"))

	cfg, err := sshConfig(state)
	if err == nil {
		t.Fatal("expected an error for a missing private key")
	}
	if cfg != nil {
		t.Errorf("expected nil ssh config, got %v", cfg)
	}
	if _, ok := state.GetOk("error"); ok {
		t.Error("callback must not record error in state; StepConnect owns that")
	}
}

func TestSSHConfigInvalidKeyReturnsError(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "bad_key")
	if err := os.WriteFile(keyPath, []byte("not a private key"), 0o600); err != nil {
		t.Fatal(err)
	}
	state := new(multistep.BasicStateBag)
	state.Put("config", Config{})
	state.Put("ui", packer.TestUi(t))
	state.Put("PRIVATE_KEY", keyPath)

	cfg, err := sshConfig(state)
	if err == nil {
		t.Fatal("expected an error for an unparsable private key")
	}
	if cfg != nil {
		t.Errorf("expected nil ssh config, got %v", cfg)
	}
	if _, ok := state.GetOk("error"); ok {
		t.Error("callback must not record error in state; StepConnect owns that")
	}
}

// winRMConfig used to return (nil, nil) when credentials could not be fetched.
// The SDK dereferences the returned config when err is nil, so that crashed the
// plugin without running Cleanup. It must now return a non-nil error.
func TestWinRMConfigCredentialFailureReturnsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"code":"internal_error","message":"boom"}]}`))
	}))
	defer srv.Close()

	id := "instance-1"
	state := new(multistep.BasicStateBag)
	state.Put("client", &IBMCloudClient{})
	state.Put("config", Config{})
	state.Put("ui", packer.TestUi(t))
	state.Put("instance_data", &vpcv1.Instance{ID: &id})
	state.Put("vpcService", newTestVpcService(t, srv.URL))

	cfg, err := winRMConfig(state)
	if err == nil {
		t.Fatal("expected an error when credentials cannot be fetched")
	}
	if cfg != nil {
		t.Errorf("expected nil winrm config alongside the error, got %v", cfg)
	}
	if _, ok := state.GetOk("error"); ok {
		t.Error("callback must not record error in state; StepConnect owns that")
	}
}

func writeTestRSAKey(t *testing.T, path string) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	if err := os.WriteFile(path, pemBytes, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestSSHConfigValidKey(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "id_rsa")
	writeTestRSAKey(t, keyPath)

	c := Config{}
	c.Comm.SSHUsername = "root"
	state := new(multistep.BasicStateBag)
	state.Put("config", c)
	state.Put("ui", packer.TestUi(t))
	state.Put("PRIVATE_KEY", keyPath)

	cfg, err := sshConfig(state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected an ssh config")
	}
	if cfg.User != "root" {
		t.Errorf("User = %q, want root", cfg.User)
	}
	if len(cfg.Auth) == 0 {
		t.Error("expected at least one auth method")
	}
	if _, ok := state.GetOk("error"); ok {
		t.Error("error must not be set on success")
	}
	if _, ok := state.GetOk("ssh_private_key"); !ok {
		t.Error("expected ssh_private_key to be stored in state")
	}
}

// StepConnect retries sshConfig until the SSH wait times out. A failed attempt
// (key not readable yet) followed by a successful one must leave no error in
// state, otherwise buildResultError would fail a build that produced an image.
func TestSSHConfigRecoversAfterTransientFailure(t *testing.T) {
	keyPath := filepath.Join(t.TempDir(), "id_rsa")

	c := Config{}
	c.Comm.SSHUsername = "root"
	state := new(multistep.BasicStateBag)
	state.Put("config", c)
	state.Put("ui", packer.TestUi(t))
	state.Put("PRIVATE_KEY", keyPath)

	if _, err := sshConfig(state); err == nil {
		t.Fatal("expected an error while the key does not exist")
	}

	writeTestRSAKey(t, keyPath)
	if _, err := sshConfig(state); err != nil {
		t.Fatalf("unexpected error on retry: %v", err)
	}

	state.Put("image_id", "img-1")
	if err := buildResultError(state); err != nil {
		t.Errorf("buildResultError = %v, want nil after a successful retry", err)
	}
}

// Same retry semantics for WinRM: the instance password is often not available
// on the first attempts, and a later attempt succeeds.
func TestWinRMConfigRecoversAfterTransientFailure(t *testing.T) {
	// Failure then success is simulated with a user-provided password, which
	// bypasses GrabCredentials, after a failing credential fetch.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"errors":[{"code":"internal_error","message":"boom"}]}`))
	}))
	defer srv.Close()

	id := "instance-1"
	state := new(multistep.BasicStateBag)
	state.Put("client", &IBMCloudClient{})
	state.Put("config", Config{})
	state.Put("ui", packer.TestUi(t))
	state.Put("instance_data", &vpcv1.Instance{ID: &id})
	state.Put("vpcService", newTestVpcService(t, srv.URL))

	if _, err := winRMConfig(state); err == nil {
		t.Fatal("expected an error on the first attempt")
	}

	c := Config{}
	c.Comm.WinRMUser = "Administrator"
	c.Comm.WinRMPassword = "secret"
	state.Put("config", c)
	cfg, err := winRMConfig(state)
	if err != nil || cfg == nil {
		t.Fatalf("expected success on retry, got cfg=%v err=%v", cfg, err)
	}

	state.Put("image_id", "img-1")
	if err := buildResultError(state); err != nil {
		t.Errorf("buildResultError = %v, want nil after a successful retry", err)
	}
}

func TestWinRMConfigUsesProvidedPassword(t *testing.T) {
	id := "instance-1"
	c := Config{}
	c.Comm.WinRMUser = "Administrator"
	c.Comm.WinRMPassword = "secret"
	state := new(multistep.BasicStateBag)
	state.Put("client", &IBMCloudClient{})
	state.Put("config", c)
	state.Put("ui", packer.TestUi(t))
	state.Put("instance_data", &vpcv1.Instance{ID: &id})

	cfg, err := winRMConfig(state)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Username != "Administrator" || cfg.Password != "secret" {
		t.Errorf("got %q/%q, want Administrator/secret", cfg.Username, cfg.Password)
	}
	if _, ok := state.GetOk("error"); ok {
		t.Error("error must not be set on success")
	}
}

// Each nil level of the response must produce an error, not a panic.
func TestGrabCredentialsIncompleteResponses(t *testing.T) {
	tests := map[string]string{
		"no password block":       `{"keys":[]}`,
		"empty password block":    `{"password":{}}`,
		"null encrypted_password": `{"password":{"encrypted_password":null}}`,
	}
	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			state := new(multistep.BasicStateBag)
			state.Put("ui", packer.TestUi(t))
			state.Put("vpcService", newTestVpcService(t, srv.URL))

			if _, _, err := (IBMCloudClient{}).GrabCredentials("instance-1", state); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}
