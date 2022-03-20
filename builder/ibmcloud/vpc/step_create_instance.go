package vpc

import (
	"context"
	"fmt"
	"time"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCreateInstance struct {
	instanceID string
}

func (step *stepCreateInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	// client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	instanceDefinition := &InstanceType{
		ResourceGroupID:  config.ResourceGroupID,
		VSIBaseImageID:   config.VSIBaseImageID,
		VSIBaseImageName: config.VSIBaseImageName,
		VSIInterface:     config.VSIInterface,
	}

	keyIDentityModel := &vpcv1.KeyIdentityByID{
		ID: &[]string{state.Get("vpc_ssh_key_id").(string)}[0],
	}
	instanceProfileIdentityModel := &vpcv1.InstanceProfileIdentityByName{
		Name: &[]string{config.VSIProfile}[0],
	}
	vpcIDentityModel := &vpcv1.VPCIdentityByID{
		ID: &[]string{state.Get("vpc_id").(string)}[0],
	}
	subnetIDentityModel := &vpcv1.SubnetIdentityByID{
		ID: &[]string{config.SubnetID}[0],
	}
	networkInterfacePrototypeModel := &vpcv1.NetworkInterfacePrototype{
		Name:   &[]string{"my-instance-modified"}[0],
		Subnet: subnetIDentityModel,
	}
	zoneIdentityModel := &vpcv1.ZoneIdentityByName{
		Name: &[]string{state.Get("zone").(string)}[0],
	}
	userData := config.VSIUserDataFile

	ui.Say("Creating Instance...")

	// Get Image ID
	if instanceDefinition.VSIBaseImageName != "" {
		ui.Say("Fetching ImageID...")
		// baseImageID, err := client.getImageIDByName(instanceDefinition.VSIBaseImageName, state)

		options := &vpcv1.ListImagesOptions{}
		options.SetName(instanceDefinition.VSIBaseImageName)
		image, _, err := vpcService.ListImages(options)

		if err != nil {
			err := fmt.Errorf("[ERROR] Error getting image with name: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		instanceDefinition.VSIBaseImageID = *image.Images[0].ID
		ui.Say(fmt.Sprintf("ImageID fetched: %s", string(instanceDefinition.VSIBaseImageID)))
	}

	imageIDentityModel := &vpcv1.ImageIdentityByID{
		ID: &[]string{instanceDefinition.VSIBaseImageID}[0],
	}
	instancePrototypeModel := &vpcv1.InstancePrototypeInstanceByImage{
		Keys:                    []vpcv1.KeyIdentityIntf{keyIDentityModel},
		Name:                    &[]string{config.VSIName}[0],
		Profile:                 instanceProfileIdentityModel,
		VPC:                     vpcIDentityModel,
		Image:                   imageIDentityModel,
		PrimaryNetworkInterface: networkInterfacePrototypeModel,
		Zone:                    zoneIdentityModel,
		UserData:                &userData,
		ResourceGroup: &vpcv1.ResourceGroupIdentityByID{
			ID: &config.ResourceGroupID,
		},
	}
	state.Put("instance_definition", *instancePrototypeModel)

	// instanceData, err := client.VPCCreateInstance(*instanceDefinition, state)
	// Start

	createInstanceOptions := vpcService.NewCreateInstanceOptions(
		instancePrototypeModel,
	)
	instanceData, _, err := vpcService.CreateInstance(createInstanceOptions)
	// End
	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating the instance: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	state.Put("instance_data", instanceData)
	ui.Say("Instance successfully created!")
	ui.Say(fmt.Sprintf("Instance's Name: %s", *instanceData.Name))
	ui.Say(fmt.Sprintf("Instance's ID: %s", *instanceData.ID))
	return multistep.ActionContinue
}

func (step *stepCreateInstance) Cleanup(state multistep.StateBag) {
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
		// status, _ := client.getStatus(floatingIPID, "floating_ips", state)

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
			// result, err := client.deleteResource(floatingIPID, "floating_ips", state)
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
	instanceData := state.Get("instance_data").(map[string]interface{})
	instanceID := instanceData["id"].(string)
	ui.Say(fmt.Sprintf("Deleting Instance ID: %s ...", instanceID))

	// result, err := client.deleteResource(instanceID, "instances", state)
	options := &vpcv1.DeleteInstanceOptions{}
	options.SetID(instanceID)
	result, err := vpcService.DeleteInstance(options)

	if err != nil {
		err := fmt.Errorf("[ERROR] Error deleting the instance. Please delete it manually: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return
	}

	if result.StatusCode == 204 {
		ui.Say("The instance was successfully deleted!")
	}

	// Deleting Security Group's rule

	ruleID := state.Get("security_group_rule_id").(string)
	ui.Say(fmt.Sprintf("Deleting Security Group's rule %s ...", ruleID))
	// resourceType := "security_groups/" + state.Get("security_group_id").(string) + "/rules"
	// result2, err2 := client.deleteResource(ruleID, resourceType, state)
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

	// Deleting Security Group
	if config.SecurityGroupID == "" {
		securityGroupName := state.Get("security_group_name").(string)
		ui.Say(fmt.Sprintf("Deleting Security Group %s ...", securityGroupName))
		securityGroupID := state.Get("security_group_id").(string)
		// result, err := client.deleteResource(securityGroupID, "security_groups", state)
		sgOptions := &vpcv1.DeleteSecurityGroupOptions{}
		options.SetID(securityGroupID)
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
