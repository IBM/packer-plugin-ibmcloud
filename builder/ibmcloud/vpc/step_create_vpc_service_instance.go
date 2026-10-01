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

const defaultTokenExchangeURL = "https://iam.cloud.ibm.com/identity/token"

// fetchAccessToken exchanges an existing IBM Cloud access token for a scoped
// token tied to desiredIAMID using the IAM iam-authz grant. exchangeURL
// defaults to the public IAM token endpoint when empty.
func fetchAccessToken(accessToken, desiredIAMID, exchangeURL string) (string, error) {
	if exchangeURL == "" {
		exchangeURL = defaultTokenExchangeURL
	}
	body := url.Values{}
	body.Set("grant_type", "urn:ibm:params:oauth:grant-type:iam-authz")
	body.Set("access_token", accessToken)
	body.Set("desired_iam_id", desiredIAMID)

	req, err := http.NewRequest(http.MethodPost, exchangeURL, strings.NewReader(body.Encode()))
	if err != nil {
		return "", fmt.Errorf("building token exchange request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("posting to token exchange endpoint %s: %w", exchangeURL, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading token exchange response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange returned HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(respBody)))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parsing token exchange response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("token exchange response contained no access_token field: %s", string(respBody))
	}
	return result.AccessToken, nil
}

// newAuthenticator returns a BearerTokenAuthenticator backed by a freshly
// exchanged scoped token when iam_access_token is configured, or an
// IamAuthenticator for the api_key path. It is called on every invocation of
// StepCreateVPCServiceInstance.Run so the token is refreshed automatically at
// both VPC service initialisation points in the pipeline.
func newAuthenticator(config Config) (core.Authenticator, error) {
	if config.IAMAccessToken != "" {
		token, err := fetchAccessToken(config.IAMAccessToken, config.IAMDesiredIAMID, config.IAMTokenExchangeURL)
		if err != nil {
			return nil, fmt.Errorf("token exchange failed: %w", err)
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
