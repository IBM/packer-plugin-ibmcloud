package vpc

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCreateSecurityGroupRules struct{}

func (s *stepCreateSecurityGroupRules) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)

	if config.SecurityGroupID == "" {
		ui.Say(fmt.Sprintf("Creating a temp Security Group on VPC %s ...", state.Get("vpc_id").(string)))

		securityGroupRequest := &SecurityGroupRequest{
			Name: config.SecurityGroupName,
			Vpc: &ResourceByID{
				Id: state.Get("vpc_id").(string),
			},
		}

		if config.ResourceGroupID != "" {
			securityGroupRequest.ResourceGroup = &ResourceByID{
				Id: config.ResourceGroupID,
			}
		}

		SecurityGroupData, err := client.createSecurityGroup(state, *securityGroupRequest)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error creating a Temp Security Group: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
		SecurityGroupID := SecurityGroupData["id"].(string)
		state.Put("security_group_id", SecurityGroupID)
		SecurityGroupName := SecurityGroupData["name"].(string)
		state.Put("security_group_name", SecurityGroupName)
		ui.Say("Temp Security Group on VPC successfully created!")
		ui.Say(fmt.Sprintf("Security Group's Name: %s", SecurityGroupName))
		ui.Say(fmt.Sprintf("Security Group's ID: %s", SecurityGroupID))

	} else {
		state.Put("security_group_id", config.SecurityGroupID)
	}

	ui.Say(fmt.Sprintf("Creating Security Group's rule to allow %s connection...", config.Comm.Type))
	var securityGroupRuleRequest = &SecurityGroupRuleRequest{}
	if config.Comm.Type == "winrm" {
		// Create rule to allow WinRM connection
		securityGroupRuleRequest = &SecurityGroupRuleRequest{
			Direction: "inbound",
			Protocol:  "tcp",
			PortMin:   5985,
			PortMax:   5986,
			IpVersion: "ipv4",
		}
	} else if config.Comm.Type == "ssh" {
		// Create rule to allow SSH connection
		securityGroupRuleRequest = &SecurityGroupRuleRequest{
			Direction: "inbound",
			Protocol:  "tcp",
			PortMin:   22,
			PortMax:   22,
			IpVersion: "ipv4",
		}
	}

	securityGroupID := state.Get("security_group_id").(string)
	ruleData, err2 := client.createRule(securityGroupID, *securityGroupRuleRequest, state)
	if err2 != nil {
		err := fmt.Errorf("[ERROR] Error creating a new Security Group's rule: %s", err2)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	ruleID := ruleData["id"].(string)
	state.Put("security_group_rule_id", ruleID)
	ui.Say(fmt.Sprintf("Security Group's rule to allow %s connection successfully created!", config.Comm.Type))
	ui.Say(fmt.Sprintf("%s rule ID: %s", config.Comm.Type, ruleID))

	// Attaching the VSI to the Security Group via primary_network_interface
	ui.Say("Attaching Instance to the Security Group")
	instanceData := state.Get("instance_data").(map[string]interface{})
	primaryNetworkInterface := instanceData["primary_network_interface"].(map[string]interface{})
	primaryNetworkInterfaceID := primaryNetworkInterface["id"].(string)

	SecurityGroupData, err := client.addNetworkInterfaceToSecurityGroup(securityGroupID, primaryNetworkInterfaceID, state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error Adding Network Interface To Security Group: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}
	SecurityGroupStatus := SecurityGroupData["status"].(string)
	ui.Say(fmt.Sprintf("Network Interface successfully added to the Security Group!: %s", SecurityGroupStatus))

	return multistep.ActionContinue
}

func (s *stepCreateSecurityGroupRules) Cleanup(state multistep.StateBag) {
	// client := state.Get("client").(*IBMCloudClient)
	// ui := state.Get("ui").(packer.Ui)
	// config := state.Get("config").(Config)
	// Security Group is deleted on `step_create_instance` Cleanup() due to a conflict when deleting Security Group: Seems the VSI becomes
	// an `Attached resources` to a Security Group, so first is required to delete the VSI, before attempting to delete the Security Group.
}
