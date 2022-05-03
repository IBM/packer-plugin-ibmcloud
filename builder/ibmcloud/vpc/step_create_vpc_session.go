package vpc

import (
	"context"
	"fmt"
	"os"

	"github.com/IBM/go-sdk-core/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCreateVPCSession struct {
}

func (step *stepCreateVPCSession) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	ui := state.Get("ui").(packer.Ui)
	iamurl := os.Getenv("AUTH_URL")
	url := os.Getenv("URL")

	ui.Say("Creating VPC service...")
	// err := client.getIAMToken(state)
	options := &vpcv1.VpcV1Options{
		Authenticator: &core.IamAuthenticator{
			ApiKey: client.IBMApiKey,
			URL:    iamurl,
		},
		URL: url,
	}
	vpcService, serviceErr := vpcv1.NewVpcV1(options)

	if serviceErr != nil {
		err := fmt.Errorf("[ERROR] Error creating VPC service %s", serviceErr)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	state.Put("vpcService", vpcService)
	ui.Say("VPC service creation successful!")
	return multistep.ActionContinue
}

func (step *stepCreateVPCSession) Cleanup(state multistep.StateBag) {
}
