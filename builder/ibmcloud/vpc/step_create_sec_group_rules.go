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
		} else if config.ResourceGroupName != "" {
			derivedResourceGroupId := state.Get("derived_resource_group_id")
			if derivedResourceGroupId != nil && derivedResourceGroupId.(string) != "" {
				derivedResourceGroupIdStr := derivedResourceGroupId.(string)
				options.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{
					ID: &derivedResourceGroupIdStr,
				}
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

	// Check if we should skip creating the default rule
	if config.SkipCreateDefaultSecurityGroupRule {
		ui.Say(fmt.Sprintf("Skipping default security group rule creation (skip_create_default_security_group_rule=true)"))
		ui.Say(fmt.Sprintf("Ensure your security group has appropriate rules for %s connectivity", config.Comm.Type))
	} else {
		// Collect all remote configurations
		type remoteConfig struct {
			remoteType string // "cidr", "address", or "id"
			value      string
		}
		var remotes []remoteConfig

		// Add CIDR blocks
		for _, cidr := range config.SecurityGroupRuleRemoteCIDR {
			remotes = append(remotes, remoteConfig{remoteType: "cidr", value: cidr})
		}

		// Add addresses
		for _, addr := range config.SecurityGroupRuleRemoteAddress {
			remotes = append(remotes, remoteConfig{remoteType: "address", value: addr})
		}

		// Add security group IDs
		for _, id := range config.SecurityGroupRuleRemoteID {
			remotes = append(remotes, remoteConfig{remoteType: "id", value: id})
		}

		// Default to allowing all IPs if no remotes specified
		if len(remotes) == 0 {
			remotes = []remoteConfig{{remoteType: "cidr", value: "0.0.0.0/0"}}
			ui.Say(fmt.Sprintf("Creating Security Group's rule to allow %s connection from all IPs (0.0.0.0/0)...", config.Comm.Type))
		} else {
			ui.Say(fmt.Sprintf("Creating Security Group's rule to allow %s connection from specified remotes...", config.Comm.Type))
		}

		// Create a rule for each remote configuration
		for i, remote := range remotes {
			securityGroupRuleRequest := &vpcv1.CreateSecurityGroupRuleOptions{}
			securityGroupRuleRequest.SetSecurityGroupID(state.Get("security_group_id").(string))

			var rulePrototype *vpcv1.SecurityGroupRulePrototypeSecurityGroupRuleProtocolTcpudp

			if config.Comm.Type == "winrm" {
				// Create rule to allow WinRM connection
				// Connection to Windows-based VSIs via WinRM
				// Protocol: TCP, Port range: 5985-5986
				rulePrototype = &vpcv1.SecurityGroupRulePrototypeSecurityGroupRuleProtocolTcpudp{
					Direction: &[]string{"inbound"}[0],
					Protocol:  &[]string{"tcp"}[0],
					PortMin:   &[]int64{5985}[0],
					PortMax:   &[]int64{5986}[0],
				}
			} else if config.Comm.Type == "ssh" {
				// Create rule to allow SSH connection
				rulePrototype = &vpcv1.SecurityGroupRulePrototypeSecurityGroupRuleProtocolTcpudp{
					Direction: &[]string{"inbound"}[0],
					Protocol:  &[]string{"tcp"}[0],
					PortMin:   &[]int64{22}[0],
					PortMax:   &[]int64{22}[0],
				}
			}

			// Set remote based on type
			switch remote.remoteType {
			case "cidr":
				rulePrototype.Remote = &vpcv1.SecurityGroupRuleRemotePrototype{
					CIDRBlock: &remote.value,
				}
			case "address":
				rulePrototype.Remote = &vpcv1.SecurityGroupRuleRemotePrototype{
					Address: &remote.value,
				}
			case "id":
				rulePrototype.Remote = &vpcv1.SecurityGroupRuleRemotePrototype{
					ID: &remote.value,
				}
			}

			securityGroupRuleRequest.SetSecurityGroupRulePrototype(rulePrototype)

			ruleData, err2 := client.createRule(*securityGroupRuleRequest, state)
			if err2 != nil {
				err := fmt.Errorf("[ERROR] Error creating a new Security Group's rule for %s %s: %s", remote.remoteType, remote.value, err2)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}

			ruleID := *ruleData.ID
			// Store the first rule ID for backward compatibility
			if i == 0 {
				state.Put("security_group_rule_id", ruleID)
			}
			ui.Say(fmt.Sprintf("Security Group's rule to allow %s connection from %s %s successfully created (ID: %s)", config.Comm.Type, remote.remoteType, remote.value, ruleID))
		}
	}

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
