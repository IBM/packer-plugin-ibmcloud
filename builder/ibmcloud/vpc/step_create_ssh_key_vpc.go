package vpc

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCreateSshKeyVPC struct{}

func (s *stepCreateSshKeyVPC) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*IBMCloudClient)

	ui.Say("Creating a new SSH key for VPC...")
	VPCSSHKeyData, err := client.createSSHKeyVPC(state)

	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating the SSH Key for VPC %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	VPCSSHKeyID := *VPCSSHKeyData.ID
	state.Put("vpc_ssh_key_id", VPCSSHKeyID)
	VPCSSHKeyName := *VPCSSHKeyData.Name
	state.Put("vpc_ssh_key_name", VPCSSHKeyName)

	ui.Say("SSH Key for VPC successfully created!")
	ui.Say(fmt.Sprintf("SSH Key for VPC's Name: %s", VPCSSHKeyName))
	ui.Say(fmt.Sprintf("SSH Key for VPC's ID: %s", VPCSSHKeyID))
	return multistep.ActionContinue
}

func (s *stepCreateSshKeyVPC) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)

	ui.Say(fmt.Sprintf("Deleting SSH key for VPC %s ...", state.Get("vpc_ssh_key_name").(string)))
	// Wait half minute before deleting SSH key - otherwise wouldn't be deleted.
	time.Sleep(30 * time.Second)
	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}
	deleteKeyOptions := &vpcv1.DeleteKeyOptions{}
	deleteKeyOptions.SetID(state.Get("vpc_ssh_key_id").(string))
	response, err := vpcService.DeleteKey(deleteKeyOptions)

	if err != nil {
		xRequestId := response.Headers["X-Request-Id"][0]
		xCorrelationId := ""
		if len(response.Headers["X-Correlation-Id"]) != 0 {
			xCorrelationId = fmt.Sprintf("\n X-Correlation-Id : %s", response.Headers["X-Correlation-Id"][0])
		}
		err := fmt.Errorf("[ERROR] Error deleting SSH key for VPC %s. Please delete it manually: %s \n X-Request-Id : %s  %s", state.Get("vpc_ssh_key_name").(string), err, xRequestId, xCorrelationId)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return
	}
	if response.StatusCode == 204 {
		ui.Say("The Key was successfully deleted!")
	} else {
		ui.Say("The key could not be deleted. Please delete it manually!")
	}
}
