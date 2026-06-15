package vpc

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseEncryptionKeyCRN(t *testing.T) {
	tests := []struct {
		name, crn                 string
		endpoint, instance, keyID string
		wantErr                   bool
	}{
		{
			name:     "key protect uses the regional endpoint",
			crn:      "crn:v1:bluemix:public:kms:us-east:a/acc:inst-1:key:key-1",
			endpoint: "https://us-east.kms.cloud.ibm.com", instance: "inst-1", keyID: "key-1",
		},
		{
			name:     "hyper protect crypto services uses the per-instance endpoint",
			crn:      "crn:v1:bluemix:public:hs-crypto:us-east:a/acc:inst-2:key:key-2",
			endpoint: "https://inst-2.api.us-east.hs-crypto.appdomain.cloud", instance: "inst-2", keyID: "key-2",
		},
		{name: "instance crn (not a key) is rejected", crn: "crn:v1:bluemix:public:kms:us-east:a/acc:inst-1::", wantErr: true},
		{name: "unsupported service is rejected", crn: "crn:v1:bluemix:public:cloud-object-storage:us-east:a/acc:inst:key:k", wantErr: true},
		{name: "garbage is rejected", crn: "not-a-crn", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpoint, instance, keyID, err := parseEncryptionKeyCRN(tt.crn)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected an error, got endpoint=%q", endpoint)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if endpoint != tt.endpoint || instance != tt.instance || keyID != tt.keyID {
				t.Fatalf("got (%q, %q, %q), want (%q, %q, %q)", endpoint, instance, keyID, tt.endpoint, tt.instance, tt.keyID)
			}
		})
	}
}

type fakeKeyVerifier struct {
	exists                           bool
	err                              error
	called                           bool
	gotEndpoint, gotInstance, gotKey string
}

func (f *fakeKeyVerifier) keyExists(endpoint, instanceID, keyID string) (bool, error) {
	f.called = true
	f.gotEndpoint, f.gotInstance, f.gotKey = endpoint, instanceID, keyID
	return f.exists, f.err
}

func TestVerifyEncryptionKeyCRN(t *testing.T) {
	const crn = "crn:v1:bluemix:public:hs-crypto:us-east:a/acc:inst-2:key:key-2"

	t.Run("key exists", func(t *testing.T) {
		f := &fakeKeyVerifier{exists: true}
		if err := verifyEncryptionKeyCRN(crn, f); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if f.gotEndpoint != "https://inst-2.api.us-east.hs-crypto.appdomain.cloud" || f.gotInstance != "inst-2" || f.gotKey != "key-2" {
			t.Fatalf("verifier got (%q, %q, %q)", f.gotEndpoint, f.gotInstance, f.gotKey)
		}
	})

	t.Run("key not found is an error", func(t *testing.T) {
		if err := verifyEncryptionKeyCRN(crn, &fakeKeyVerifier{exists: false}); err == nil {
			t.Fatal("expected an error when the key is not found")
		}
	})

	t.Run("verifier error is propagated", func(t *testing.T) {
		if err := verifyEncryptionKeyCRN(crn, &fakeKeyVerifier{err: fmt.Errorf("boom")}); err == nil {
			t.Fatal("expected the verifier error to propagate")
		}
	})

	t.Run("malformed crn short-circuits before the verifier", func(t *testing.T) {
		f := &fakeKeyVerifier{exists: true}
		if err := verifyEncryptionKeyCRN("garbage", f); err == nil {
			t.Fatal("expected a parse error")
		}
		if f.called {
			t.Fatal("verifier must not be called for a malformed CRN")
		}
	})
}

// stubAuthenticator is a no-op core.Authenticator for exercising kmsKeyVerifier against httptest.
type stubAuthenticator struct{ err error }

func (s stubAuthenticator) AuthenticationType() string       { return "stub" }
func (s stubAuthenticator) Validate() error                  { return nil }
func (s stubAuthenticator) Authenticate(*http.Request) error { return s.err }

func TestKMSKeyVerifierKeyExists(t *testing.T) {
	tests := []struct {
		name       string
		status     int
		body       string
		wantExists bool
		wantErr    bool
	}{
		{name: "200 means the key exists", status: http.StatusOK, body: `{"resources":[{"id":"key-1"}]}`, wantExists: true},
		{name: "404 means absent without an error", status: http.StatusNotFound, body: `{}`},
		{name: "403 is an error, not absent", status: http.StatusForbidden, body: `{"resources":[{"errorMsg":"denied"}]}`, wantErr: true},
		{name: "401 is an error, not absent", status: http.StatusUnauthorized, body: `{}`, wantErr: true},
		{name: "500 is an error", status: http.StatusInternalServerError, body: "boom", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotPath, gotAccept, gotInstance string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotPath = r.URL.Path
				gotAccept = r.Header.Get("Accept")
				gotInstance = r.Header.Get("Bluemix-Instance")
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			v := kmsKeyVerifier{authenticator: stubAuthenticator{}, client: srv.Client()}
			exists, err := v.keyExists(srv.URL, "inst-9", "key-1")

			if (err != nil) != tt.wantErr {
				t.Fatalf("keyExists error = %v, wantErr = %v", err, tt.wantErr)
			}
			if exists != tt.wantExists {
				t.Fatalf("keyExists = %v, want %v", exists, tt.wantExists)
			}
			if gotPath != "/api/v2/keys/key-1" {
				t.Errorf("request path = %q, want /api/v2/keys/key-1", gotPath)
			}
			if gotInstance != "inst-9" {
				t.Errorf("Bluemix-Instance header = %q, want inst-9", gotInstance)
			}
			if gotAccept != "application/vnd.ibm.kms.key+json" {
				t.Errorf("Accept header = %q, want application/vnd.ibm.kms.key+json", gotAccept)
			}
		})
	}
}

func TestKMSKeyVerifierAuthenticateError(t *testing.T) {
	v := kmsKeyVerifier{authenticator: stubAuthenticator{err: fmt.Errorf("no token")}, client: http.DefaultClient}
	if _, err := v.keyExists("https://example.invalid", "inst-9", "key-1"); err == nil {
		t.Fatal("expected an error when authentication fails")
	}
}
