package vpc

import (
	"context"
	"fmt"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCreateInstance struct{}

func (step *stepCreateInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	instanceDefinition := &InstanceType{
		EndPoint:         config.EndPoint,
		Version:          config.Version,
		Generation:       config.Generation,
		Zone:             state.Get("zone").(string),
		VPCID:            state.Get("vpc_id").(string),
		SubnetID:         config.SubnetID,
		ResourceGroupID:  config.ResourceGroupID,
		VPCSSHKeyID:      state.Get("vpc_ssh_key_id").(string),
		VSIName:          config.VSIName,
		VSIBaseImageID:   config.VSIBaseImageID,
		VSIBaseImageName: config.VSIBaseImageName,
		VSIProfile:       config.VSIProfile,
		VSIInterface:     config.VSIInterface,
		VSIUserDataFile:  config.VSIUserDataFile,
	}
	state.Put("instance_definition", *instanceDefinition)

	ui.Say("Creating Instance...")
	// Fetching Base Image ID
	if instanceDefinition.VSIBaseImageName != "" {
		instanceDefinition.VSIBaseImageID = state.Get("baseImageID").(string)
	}

	instanceData, err := client.VPCCreateInstance(*instanceDefinition, state)
	if err != nil || instanceData == nil {
		err := fmt.Errorf("[ERROR] Error creating the instance: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	state.Put("instance_data", instanceData)
	ui.Say("Instance successfully created!")
	ui.Say(fmt.Sprintf("Instance's Name: %s", instanceData["name"].(string)))
	ui.Say(fmt.Sprintf("Instance's ID: %s", instanceData["id"].(string)))
	return multistep.ActionContinue
}

func (step *stepCreateInstance) Cleanup(state multistep.StateBag) {
	config := state.Get("config").(Config)
	client := state.Get("client").(*IBMCloudClient)
	ui := state.Get("ui").(packer.Ui)

	// Delete Floating IP if it was created (VSI Interface was set as public)
	if config.VSIInterface == "public" {
		floatingIP := state.Get("floating_ip").(string)
		ui.Say(fmt.Sprintf("Releasing the Floating IP: %s ...", floatingIP))

		floatingIPID := state.Get("floating_ip_id").(string)
		status, _ := client.getStatus(floatingIPID, "floating_ips", state)

		if status == "available" {
			result, err := client.deleteResource(floatingIPID, "floating_ips", state)
			if err != nil {
				err := fmt.Errorf("[ERROR] Error releasing the Floating IP. Please release it manually: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				// log.Fatalf(err.Error())
				return
			}
			if result == "204 No Content" {
				ui.Say("The Floating IP was successfully released!")
			}
		}
	}

	// Wait a couple of seconds before attempting to delete the instance.
	time.Sleep(2 * time.Second)
	instanceData := state.Get("instance_data").(map[string]interface{})
	instanceID := instanceData["id"].(string)
	ui.Say(fmt.Sprintf("Deleting Instance ID: %s ...", instanceID))

	result, err := client.deleteResource(instanceID, "instances", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error deleting the instance. Please delete it manually: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return
	}

	if result == "204 No Content" {
		ui.Say("The instance was successfully deleted!")
	}

	// Deleting Security Group's rule
	ruleID := state.Get("security_group_rule_id").(string)
	ui.Say(fmt.Sprintf("Deleting Security Group's rule %s ...", ruleID))
	resourceType := "security_groups/" + state.Get("security_group_id").(string) + "/rules"
	result2, err2 := client.deleteResource(ruleID, resourceType, state)
	if err2 != nil {
		err2 := fmt.Errorf("[ERROR] Error deleting Security Group's rule %s. Please delete it manually: %s", ruleID, err2)
		state.Put("error", err2)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return
	}

	if result2 == "204 No Content" {
		ui.Say("The Security Group's rule was successfully deleted!")
	}

	// Deleting Security Group
	if config.SecurityGroupID == "" {
		securityGroupName := state.Get("security_group_name").(string)
		ui.Say(fmt.Sprintf("Deleting Security Group %s ...", securityGroupName))
		securityGroupID := state.Get("security_group_id").(string)
		result, err := client.deleteResource(securityGroupID, "security_groups", state)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error deleting Security Group %s. Please delete it manually: %s", securityGroupName, err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return
		}

		if result == "204 No Content" {
			ui.Say("The Security Group was successfully deleted!")
		}
	}

}
