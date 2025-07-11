package vpc

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepWaitforInstance struct{}

func (s *stepWaitforInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	ui.Say("Waiting for the instance to become ACTIVE...")
	instanceData := state.Get("instance_data").(*vpcv1.Instance)
	instanceID := *instanceData.ID
	err := client.waitForResourceReady(instanceID, "instances", config.StateTimeout, state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error step waiting for instance to become ACTIVE: %s", err.Error())
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	// Update instance_data with new information unavailable at creation time (Private_IP, etc..)
	newInstanceData, _ := client.retrieveResource(instanceID, state)
	state.Put("instance_data", newInstanceData)
	ui.Say("Instance is ACTIVE!")
	return multistep.ActionContinue
}

func (client *stepWaitforInstance) Cleanup(state multistep.StateBag) {
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)
	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	// Delete Floating IP if it was created (VSI Interface was set as public)
	if config.VSIInterface == "public" {
		floatingIP := state.Get("floating_ip").(string)
		ui.Say(fmt.Sprintf("Releasing the Floating IP: %s ...", floatingIP))

		floatingIPID := state.Get("floating_ip_id").(string)

		options := vpcService.NewGetFloatingIPOptions(floatingIPID)
		floatingIPresponse, _, err := vpcService.GetFloatingIP(options)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error getting the Floating IP: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return
		}
		status := floatingIPresponse.Status
		if *status == "available" {
			options := vpcService.NewDeleteFloatingIPOptions(floatingIPID)
			result, err := vpcService.DeleteFloatingIP(options)

			if err != nil {
				err := fmt.Errorf("[ERROR] Error releasing the Floating IP. Please release it manually: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				// log.Fatalf(err.Error())
				return
			}
			if result.StatusCode == 204 {
				ui.Say("The Floating IP was successfully released!")
			}
		}
	}

	// Wait a couple of seconds before attempting to delete the instance.
	time.Sleep(2 * time.Second)
	instanceData := state.Get("instance_data").(*vpcv1.Instance)
	instanceID := *instanceData.ID
	ui.Say(fmt.Sprintf("Deleting Instance ID: %s ...", instanceID))

	options := &vpcv1.DeleteInstanceOptions{}
	options.SetID(instanceID)
	_, err := vpcService.DeleteInstance(options)

	if err != nil {
		err := fmt.Errorf("[ERROR] Error deleting the instance. Please delete it manually: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return
	}
	instanceDeleted := false
	for !instanceDeleted {
		options := &vpcv1.GetInstanceOptions{}
		options.SetID(instanceID)
		instance, response, err := vpcService.GetInstance(options)
		if err != nil {
			if response != nil && response.StatusCode == 404 {
				ui.Say("Instance deleted Succesfully")
				instanceDeleted = true
				break
			}
			err := fmt.Errorf("[ERROR] Error getting the instance to check delete status. %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
		} else if instance != nil {
			ui.Say(fmt.Sprintf("Instance status :-  %s", *instance.Status))
		}
		time.Sleep(10 * time.Second)
	}

	// Deleting Security Group's rule

	ruleID := state.Get("security_group_rule_id").(string)
	ui.Say(fmt.Sprintf("Deleting Security Group's rule %s ...", ruleID))
	sgRuleOptions := &vpcv1.DeleteSecurityGroupRuleOptions{}
	sgRuleOptions.SetSecurityGroupID(state.Get("security_group_id").(string))
	sgRuleOptions.SetID(ruleID)
	sgRuleResponse, sgRuleErr := vpcService.DeleteSecurityGroupRule(sgRuleOptions)

	if sgRuleErr != nil {
		sgRuleErr := fmt.Errorf("[ERROR] Error deleting Security Group's rule %s. Please delete it manually: %s", ruleID, sgRuleErr)
		state.Put("error", sgRuleErr)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return
	}

	if sgRuleResponse.StatusCode == 204 {
		ui.Say("The Security Group's rule was successfully deleted!")
	}

	// Wait a couple of seconds before attempting to delete the security group.
	time.Sleep(10 * time.Second)

	// Deleting Security Group
	if config.SecurityGroupID == "" {
		securityGroupName := state.Get("security_group_name").(string)
		ui.Say(fmt.Sprintf("Deleting Security Group %s ...", securityGroupName))
		securityGroupID := state.Get("security_group_id").(string)
		sgOptions := &vpcv1.DeleteSecurityGroupOptions{}
		sgOptions.SetID(securityGroupID)
		sgResponse, err := vpcService.DeleteSecurityGroup(sgOptions)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error deleting Security Group %s. Please delete it manually: %s", securityGroupName, err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return
		}

		if sgResponse.StatusCode == 204 {
			ui.Say("The Security Group was successfully deleted!")
		}
	}
}
