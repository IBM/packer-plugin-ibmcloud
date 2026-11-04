package classic

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/pem"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// roundTripFunc lets a test stub the SoftLayer API without network access.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestStepCreateSshKey(t *testing.T) {
	const apiKey = "super-secret-api-key"

	// The step writes to ./ssh_keys, so run inside a temp dir (restored automatically).
	t.Chdir(t.TempDir())

	var uploadedBody string
	client := &SoftlayerClient{
		user:   "test-user",
		apiKey: apiKey,
		http: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			body := "true"
			if r.Method == "POST" {
				b, _ := io.ReadAll(r.Body)
				uploadedBody = string(b)
				body = `{"id": 4242}`
			}
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
		})},
	}

	var logBuf bytes.Buffer
	log.SetOutput(&logBuf)
	defer log.SetOutput(os.Stderr)

	state := new(multistep.BasicStateBag)
	state.Put("ui", packer.TestUi(t))
	state.Put("client", client)

	step := &stepCreateSshKey{}
	if action := step.Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("expected ActionContinue, got %v (error: %v)", action, state.Get("error"))
	}

	// Key id propagated.
	if step.keyId != 4242 {
		t.Errorf("keyId = %d, want 4242", step.keyId)
	}

	// Private key must be 2048-bit RSA (regression: was 2014).
	block, _ := pem.Decode([]byte(state.Get("ssh_private_key").(string)))
	if block == nil {
		t.Fatal("ssh_private_key is not valid PEM")
	}
	priv, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	if got := priv.N.BitLen(); got != 2048 {
		t.Errorf("RSA key size = %d, want 2048", got)
	}

	// Files exist with 0600.
	for _, f := range []string{"ssh_keys/id_rsa", "ssh_keys/id_rsa.pub"} {
		fi, err := os.Stat(f)
		if err != nil {
			t.Fatalf("%s missing: %v", f, err)
		}
		if fi.Mode().Perm() != 0600 {
			t.Errorf("%s mode = %v, want 0600", f, fi.Mode().Perm())
		}
	}

	// Public key was sent to the API.
	if !strings.Contains(uploadedBody, "ssh-rsa ") {
		t.Errorf("uploaded body does not contain an ssh-rsa public key: %s", uploadedBody)
	}

	// Sensitive-log regression: API key must never appear in logs.
	if strings.Contains(logBuf.String(), apiKey) {
		t.Errorf("API key leaked into logs:\n%s", logBuf.String())
	}

	// Cleanup removes the directory.
	step.Cleanup(state)
	if _, err := os.Stat("ssh_keys"); !os.IsNotExist(err) {
		t.Errorf("ssh_keys directory still exists after Cleanup")
	}
}
