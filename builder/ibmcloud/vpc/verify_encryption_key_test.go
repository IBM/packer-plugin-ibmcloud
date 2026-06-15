package vpc

import (
	"fmt"
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
