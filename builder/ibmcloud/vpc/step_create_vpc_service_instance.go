package vpc

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// vpcRetryMaxAttempts and vpcRetryMaxInterval configure the IBM Cloud SDK's
// built-in request retries (go-sdk-core EnableRetries). The SDK retries 429 and
// 5xx (except 501) responses and network-level failures, honors the Retry-After
// header, and backs off exponentially up to vpcRetryMaxInterval. This rides out
// transient API blips on every VPC call — both one-shot creates and status
// polls — so a single 502 no longer aborts an otherwise-healthy bake.
const (
	vpcRetryMaxAttempts = 5
	vpcRetryMaxInterval = 30 * time.Second
)

type StepCreateVPCServiceInstance struct {
}

func (step *StepCreateVPCServiceInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
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

	authenticator := &core.IamAuthenticator{
		ApiKey: client.IBMApiKey,
		URL:    config.IAMEndpoint,
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
