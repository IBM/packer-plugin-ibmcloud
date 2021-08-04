package vpc

import (
	"context"
	"fmt"

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

	VPCSSHKeyID := VPCSSHKeyData["id"].(string)
	state.Put("vpc_ssh_key_id", VPCSSHKeyID)
	VPCSSHKeyName := VPCSSHKeyData["name"].(string)
	state.Put("vpc_ssh_key_name", VPCSSHKeyName)

	ui.Say("SSH Key for VPC successfully created!")
	ui.Say(fmt.Sprintf("SSH Key for VPC's Name: %s", VPCSSHKeyName))
	ui.Say(fmt.Sprintf("SSH Key for VPC's ID: %s", VPCSSHKeyID))
	return multistep.ActionContinue
}

func (s *stepCreateSshKeyVPC) Cleanup(state multistep.StateBag) {
	client := state.Get("client").(*IBMCloudClient)
	ui := state.Get("ui").(packer.Ui)

	ui.Say(fmt.Sprintf("Deleting SSH key for VPC %s ...", state.Get("vpc_ssh_key_name").(string)))
	// Wait half minute before deleting SSH key - otherwise wouldn't be deleted.
	// time.Sleep(30 * time.Second)
	result, err := client.deleteResource(state.Get("vpc_ssh_key_id").(string), "keys", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error deleting SSH key for VPC %s. Please delete it manually: %s", state.Get("vpc_ssh_key_name").(string), err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return
	}
	if result == "204 No Content" {
		ui.Say("The Key was successfully deleted!")
	}
}
