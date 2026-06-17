package vpc

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
)

// keyVerifier reports whether the KMS key referenced by an encryption_key_crn exists and is
// readable. It is a seam so the verification can be faked in tests.
type keyVerifier interface {
	keyExists(endpoint, instanceID, keyID string) (bool, error)
}

// parseEncryptionKeyCRN derives the KMS API endpoint, instance id, and key id from a Key Protect
// or Hyper Protect Crypto Services key CRN. Key Protect keys are served from the regional KMS
// endpoint; Hyper Protect Crypto Services keys are served from the per-instance endpoint.
//
// Expected CRN shape:
//
//	crn:v1:bluemix:public:<service>:<region>:a/<account>:<instance>:key:<keyID>
func parseEncryptionKeyCRN(crn string) (endpoint, instanceID, keyID string, err error) {
	p := strings.Split(crn, ":")
	if len(p) < 10 || p[0] != "crn" || p[8] != "key" {
		return "", "", "", fmt.Errorf("not a recognized Key Protect / Hyper Protect Crypto Services key CRN: %q", crn)
	}
	service, region, instanceID, keyID := p[4], p[5], p[7], p[9]
	if region == "" || instanceID == "" || keyID == "" {
		return "", "", "", fmt.Errorf("encryption key CRN is missing a region, instance, or key id: %q", crn)
	}
	switch service {
	case "hs-crypto": // Hyper Protect Crypto Services: per-instance endpoint
		endpoint = fmt.Sprintf("https://%s.api.%s.hs-crypto.appdomain.cloud", instanceID, region)
	case "kms": // Key Protect: regional endpoint
		endpoint = fmt.Sprintf("https://%s.kms.cloud.ibm.com", region)
	default:
		return "", "", "", fmt.Errorf("unsupported KMS service %q in encryption key CRN: %q", service, crn)
	}
	return endpoint, instanceID, keyID, nil
}

// kmsKeyVerifier checks for a key through the Key Protect-compatible KMS API.
type kmsKeyVerifier struct {
	authenticator core.Authenticator
	client        *http.Client
}

var _ keyVerifier = kmsKeyVerifier{}

// newKMSKeyVerifier builds a kmsKeyVerifier that authenticates with the build's IBM Cloud API key.
func newKMSKeyVerifier(apiKey, iamURL string) kmsKeyVerifier {
	return kmsKeyVerifier{
		authenticator: &core.IamAuthenticator{ApiKey: apiKey, URL: iamURL},
		client:        &http.Client{Timeout: 30 * time.Second},
	}
}

// keyExists reports whether a key with keyID is present in the KMS instance. It LISTs the
// instance's keys (GET /api/v2/keys) and matches by id rather than GET /api/v2/keys/{id}: reading
// a specific key requires a higher privilege than listing, and the build identity only needs to
// confirm the key exists — the boot-volume encryption itself is performed by the VPC service via a
// Key Protect service-to-service authorization, not by this identity.
func (v kmsKeyVerifier) keyExists(endpoint, instanceID, keyID string) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, endpoint+"/api/v2/keys", nil)
	if err != nil {
		return false, fmt.Errorf("building KMS list-keys request: %w", err)
	}
	req.Header.Set("Bluemix-Instance", instanceID)
	if err := v.authenticator.Authenticate(req); err != nil {
		return false, fmt.Errorf("authenticating KMS request: %w", err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("contacting KMS at %s to list keys: %w", endpoint, err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	switch resp.StatusCode {
	case http.StatusOK:
		// parsed below
	case http.StatusUnauthorized, http.StatusForbidden:
		return false, fmt.Errorf("KMS denied listing keys at %s (HTTP %d) — the build's API key likely lacks reader access to this Key Protect / Hyper Protect Crypto Services instance: %s",
			endpoint, resp.StatusCode, oneLineSnippet(body))
	default:
		return false, fmt.Errorf("KMS list keys at %s returned HTTP %d: %s", endpoint, resp.StatusCode, oneLineSnippet(body))
	}
	var out struct {
		Resources []struct {
			ID string `json:"id"`
		} `json:"resources"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return false, fmt.Errorf("parsing KMS list-keys response from %s: %w", endpoint, err)
	}
	for _, r := range out.Resources {
		if r.ID == keyID {
			return true, nil
		}
	}
	return false, nil
}

// oneLineSnippet collapses a (response body) byte slice to a single-line string for error messages.
func oneLineSnippet(b []byte) string {
	return strings.Join(strings.Fields(string(b)), " ")
}

// verifyEncryptionKeyCRN validates an encryption_key_crn by confirming the key exists via the KMS API.
func verifyEncryptionKeyCRN(crn string, v keyVerifier) error {
	endpoint, instanceID, keyID, err := parseEncryptionKeyCRN(crn)
	if err != nil {
		return err
	}
	exists, err := v.keyExists(endpoint, instanceID, keyID)
	if err != nil {
		return err
	}
	if !exists {
		return fmt.Errorf("key %s not found in the KMS instance at %s", keyID, endpoint)
	}
	return nil
}
