package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
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

		vpc_id := state.Get("vpc_id").(string)

		options := &vpcv1.CreateSecurityGroupOptions{}
		options.SetVPC(&vpcv1.VPCIdentity{
			ID: &vpc_id,
		})
		options.SetName(config.SecurityGroupName)

		if config.ResourceGroupID != "" {
			options.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{
				ID: &config.ResourceGroupID,
			}
		}

		SecurityGroupData, err := client.createSecurityGroup(state, *options)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error creating a Temp Security Group: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
		securityGroupID := *SecurityGroupData.ID
		state.Put("security_group_id", securityGroupID)
		securityGroupName := *SecurityGroupData.Name
		state.Put("security_group_name", securityGroupName)
		ui.Say("Temp Security Group on VPC successfully created!")
		ui.Say(fmt.Sprintf("Security Group's Name: %s", securityGroupName))
		ui.Say(fmt.Sprintf("Security Group's ID: %s", securityGroupID))

	} else {
		state.Put("security_group_id", config.SecurityGroupID)
		ui.Say(fmt.Sprintf("Looking for security group: %s", config.SecurityGroupID))
		options := &vpcv1.GetSecurityGroupOptions{}
		options.SetID(config.SecurityGroupID)
		SecurityGroupData, err := client.getSecurityGroup(state, *options)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error getting Security Group: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		securityGroupName := *SecurityGroupData.Name
		securityGroupID := *SecurityGroupData.ID
		ui.Say(fmt.Sprintf("Security group with name %s and ID %s found.", securityGroupName, securityGroupID))
	}

	ui.Say(fmt.Sprintf("Creating Security Group's rule to allow %s connection...", config.Comm.Type))
	securityGroupRuleRequest := &vpcv1.CreateSecurityGroupRuleOptions{}
	securityGroupRuleRequest.SetSecurityGroupID(state.Get("security_group_id").(string))

	if config.Comm.Type == "winrm" {
		// Create rule to allow WinRM connection
		// Connection to Windows-based VSIs via WinRM
		// Protocol: TCP, Port range: 5985-5986, Source Type: Any
		securityGroupRuleRequest.SetSecurityGroupRulePrototype(&vpcv1.SecurityGroupRulePrototypeSecurityGroupRuleProtocolTcpudp{
			Direction: &[]string{"inbound"}[0],
			Protocol:  &[]string{"tcp"}[0],
			PortMin:   &[]int64{5985}[0],
			PortMax:   &[]int64{5986}[0],
		})
	} else if config.Comm.Type == "ssh" {
		// Create rule to allow SSH connection
		securityGroupRuleRequest.SetSecurityGroupRulePrototype(&vpcv1.SecurityGroupRulePrototypeSecurityGroupRuleProtocolTcpudp{
			Direction: &[]string{"inbound"}[0],
			Protocol:  &[]string{"tcp"}[0],
			PortMin:   &[]int64{22}[0],
			PortMax:   &[]int64{22}[0],
		})
	}

	ruleData, err2 := client.createRule(*securityGroupRuleRequest, state)
	if err2 != nil {
		err := fmt.Errorf("[ERROR] Error creating a new Security Group's rule: %s", err2)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	ruleID := *ruleData.ID
	state.Put("security_group_rule_id", ruleID)
	ui.Say(fmt.Sprintf("Security Group's rule to allow %s connection successfully created!", config.Comm.Type))

	// Attaching the VSI to the Security Group via primary_network_interface
	ui.Say("Attaching Instance to the Security Group")
	instanceData := state.Get("instance_data").(*vpcv1.Instance)
	primaryNetworkInterfaceID := *instanceData.PrimaryNetworkInterface.ID
	_, err := client.addNetworkInterfaceToSecurityGroup(state.Get("security_group_id").(string), primaryNetworkInterfaceID, state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error Adding Network Interface To Security Group: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}
	ui.Say("Instance successfully added to the Security Group.")

	return multistep.ActionContinue
}

func (s *stepCreateSecurityGroupRules) Cleanup(state multistep.StateBag) {
	// client := state.Get("client").(*IBMCloudClient)
	// ui := state.Get("ui").(packer.Ui)
	// config := state.Get("config").(Config)
	// Security Group is deleted on `step_create_instance` Cleanup() due to a conflict when deleting Security Group: Seems the VSI becomes
	// an `Attached resources` to a Security Group, so first is required to delete the VSI, before attempting to delete the Security Group.
}
