package vpc

import (
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

// kmsKeyVerifier reads a key through the Key Protect-compatible KMS API (GET /api/v2/keys/{id}).
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

// keyExists reports whether the key is readable. 200 -> exists; 404 -> absent (or not visible to
// this identity). Any other status is returned as an error so the build halts rather than
// misreporting an auth/transport failure as a missing key; 401/403 are called out specifically
// since they almost always mean the build's API key lacks reader access to the KMS instance.
func (v kmsKeyVerifier) keyExists(endpoint, instanceID, keyID string) (bool, error) {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("%s/api/v2/keys/%s", endpoint, keyID), nil)
	if err != nil {
		return false, fmt.Errorf("building KMS request for key %s: %w", keyID, err)
	}
	req.Header.Set("Accept", "application/vnd.ibm.kms.key+json")
	req.Header.Set("Bluemix-Instance", instanceID)
	if err := v.authenticator.Authenticate(req); err != nil {
		return false, fmt.Errorf("authenticating KMS request for key %s: %w", keyID, err)
	}
	resp, err := v.client.Do(req)
	if err != nil {
		return false, fmt.Errorf("contacting KMS at %s to verify key %s: %w", endpoint, keyID, err)
	}
	defer resp.Body.Close()
	switch resp.StatusCode {
	case http.StatusOK:
		return true, nil
	case http.StatusNotFound:
		return false, nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return false, fmt.Errorf("KMS denied access to key %s at %s (HTTP %d) — the build's API key likely lacks reader access to this Key Protect / Hyper Protect Crypto Services instance: %s",
			keyID, endpoint, resp.StatusCode, kmsBodySnippet(resp))
	default:
		return false, fmt.Errorf("KMS GET key %s at %s returned HTTP %d: %s", keyID, endpoint, resp.StatusCode, kmsBodySnippet(resp))
	}
}

// kmsBodySnippet returns a bounded, single-line snippet of the response body so KMS's own error
// message (e.g. resources[].errorMsg) shows up in the build log.
func kmsBodySnippet(resp *http.Response) string {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
	return strings.Join(strings.Fields(string(b)), " ")
}

// verifyEncryptionKeyCRN validates an encryption_key_crn by reading the key through the KMS API.
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
		return fmt.Errorf("key %s not found or not accessible at %s", keyID, endpoint)
	}
	return nil
}
