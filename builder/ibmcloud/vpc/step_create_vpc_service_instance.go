package vpc

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// vpcRetryMaxAttempts and vpcRetryMaxInterval configure the IBM Cloud SDK's
// built-in request retries (go-sdk-core EnableRetries). The SDK retries 429 and
// 5xx (except 501) responses and network-level failures. It honors a server-sent
// Retry-After header (as given); otherwise it backs off exponentially, with
// vpcRetryMaxInterval capping that exponential wait. This rides out transient API
// blips on every VPC call — both one-shot creates and status polls — so a single
// 502 no longer aborts an otherwise-healthy bake.
const (
	vpcRetryMaxAttempts = 5
	vpcRetryMaxInterval = 30 * time.Second
)

// iamPost is the shared helper for both IAM token grant types. It POSTs body
// to exchangeURL and returns the access_token field from the JSON response.
func iamPost(body url.Values, exchangeURL, errPrefix string) (string, error) {
	req, err := http.NewRequest(http.MethodPost, exchangeURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", fmt.Errorf("%s: building request: %w", errPrefix, err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%s: posting to %s: %w", errPrefix, exchangeURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("%s: reading response: %w", errPrefix, err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s: HTTP %d: %s", errPrefix, resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("%s: parsing response: %w", errPrefix, err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("%s: response contained no access_token field: %s", errPrefix, string(respBody))
	}
	return result.AccessToken, nil
}

// fetchTokenFromAPIKey exchanges a Service ID API key for a plain IBM Cloud
// access token using the apikey grant type (Step 1 of the two-step delegation
// flow). The resulting token is passed to fetchDelegationToken (Step 2).
func fetchTokenFromAPIKey(apiKey, exchangeURL string) (string, error) {
	body := url.Values{}
	body.Set("grant_type", "urn:ibm:params:oauth:grant-type:apikey")
	body.Set("apikey", apiKey)
	return iamPost(body, exchangeURL, "apikey token fetch")
}

// fetchDelegationToken exchanges an intermediate access token for a scoped
// delegation token tied to desiredIAMID using the iam-authz grant (Step 2).
func fetchDelegationToken(accessToken, desiredIAMID, exchangeURL string) (string, error) {
	body := url.Values{}
	body.Set("grant_type", "urn:ibm:params:oauth:grant-type:iam-authz")
	body.Set("access_token", accessToken)
	body.Set("desired_iam_id", desiredIAMID)
	return iamPost(body, exchangeURL, "iam-authz token exchange")
}
// exchangeURLFromIAMEndpoint derives the /identity/token endpoint from the
// configured IAM base URL (iam_url). IAMEndpoint is always set by Prepare(),
// so iamEndpoint is guaranteed non-empty at runtime.
func exchangeURLFromIAMEndpoint(iamEndpoint string) string {
	return strings.TrimRight(iamEndpoint, "/") + "/identity/token"
}



// newAuthenticator returns a BearerTokenAuthenticator backed by a freshly
// derived scoped token when iam_service_api_key is configured, or an
// IamAuthenticator for the api_key path. It is called on every invocation of
// StepCreateVPCServiceInstance.Run so the token is always fresh at both VPC
// service initialisation points in the pipeline.
func newAuthenticator(config Config) (core.Authenticator, error) {
	if config.IAMServiceAPIKey != "" {
		exchangeURL := exchangeURLFromIAMEndpoint(config.IAMEndpoint)
		// Step 1: Service ID API key → intermediate access token.
		accessToken, err := fetchTokenFromAPIKey(config.IAMServiceAPIKey, exchangeURL)
		if err != nil {
			return nil, fmt.Errorf("failed to obtain access token from iam_service_api_key: %w", err)
		}
		// Step 2: intermediate access token → scoped delegation token.
		token, err := fetchDelegationToken(accessToken, config.DesiredIAMID, exchangeURL)
		if err != nil {
			return nil, fmt.Errorf("Failed to obtain delegation token for desired_iam_id: %w", err)
		}
		return &core.BearerTokenAuthenticator{BearerToken: token}, nil
	}
	return &core.IamAuthenticator{
		ApiKey: config.IBMApiKey,
		URL:    config.IAMEndpoint,
	}, nil
}

type StepCreateVPCServiceInstance struct {
}

func (step *StepCreateVPCServiceInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)

	ui.Say("Creating VPC service...")

	// Enable logging for IBM Cloud Go SDK core based on logging configuration
	if config.VPCLog != "" {
		var logLevel core.LogLevel
		switch config.VPCLog {
		case "error":
			logLevel = core.LevelError
		case "warn":
			logLevel = core.LevelWarn
		case "info":
			logLevel = core.LevelInfo
		case "debug":
			logLevel = core.LevelDebug
		default:
			ui.Error(fmt.Sprintf("Invalid logging value '%s'. Valid values are: error, warn, info, debug", config.VPCLog))
			logLevel = core.LevelError
		}

		logDestination := log.Writer()
		goLogger := log.New(logDestination, "", log.LstdFlags)
		core.SetLogger(core.NewLogger(logLevel, goLogger, goLogger))
	}

	authenticator, authErr := newAuthenticator(config)
	if authErr != nil {
		err := fmt.Errorf("[ERROR] Authentication setup failed: %s", authErr)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	options := &vpcv1.VpcV1Options{
		Authenticator: authenticator,
		URL:           config.Endpoint,
	}
	vpcService, serviceErr := vpcv1.NewVpcV1(options)

	if serviceErr != nil {
		err := fmt.Errorf("[ERROR] Error creating VPC service %s", serviceErr)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	vpcService.EnableRetries(vpcRetryMaxAttempts, vpcRetryMaxInterval)

	state.Put("vpcService", vpcService)
	ui.Say("VPC service creation successful!")
	return multistep.ActionContinue
}

func (step *StepCreateVPCServiceInstance) Cleanup(state multistep.StateBag) {
}
