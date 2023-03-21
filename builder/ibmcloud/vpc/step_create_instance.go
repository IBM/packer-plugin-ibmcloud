package vpc

import (
	"context"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCreateInstance struct{}

func (step *stepCreateInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	vsiBaseImageName := config.VSIBaseImageName
	vsiBaseImageID := config.VSIBaseImageID
	vsiCatalogOfferingCrn := config.CatalogOfferingCRN
	vsiCatalogOfferingVersionCrn := config.CatalogOfferingVersionCRN

	keyIdentityModel := &vpcv1.KeyIdentityByID{
		ID: &[]string{state.Get("vpc_ssh_key_id").(string)}[0],
	}
	instanceProfileIdentityModel := &vpcv1.InstanceProfileIdentityByName{
		Name: &[]string{config.VSIProfile}[0],
	}
	vpcIdentityModel := &vpcv1.VPCIdentityByID{
		ID: &[]string{state.Get("vpc_id").(string)}[0],
	}
	subnetIdentityModel := &vpcv1.SubnetIdentityByID{
		ID: &[]string{config.SubnetID}[0],
	}
	networkInterfacePrototypeModel := &vpcv1.NetworkInterfacePrototype{
		Name:   &[]string{"my-instance-modified"}[0],
		Subnet: subnetIdentityModel,
	}
	zoneIdentityModel := &vpcv1.ZoneIdentityByName{
		Name: &[]string{state.Get("zone").(string)}[0],
	}

	ui.Say("Creating Instance...")

	// For catalog images
	if vsiCatalogOfferingCrn != "" || vsiCatalogOfferingVersionCrn != "" {

		catalogOfferingPrototype := &vpcv1.InstanceCatalogOfferingPrototype{}

		// offering crn
		if vsiCatalogOfferingCrn != "" {
			offering := &vpcv1.CatalogOfferingIdentityCatalogOfferingByCRN{
				CRN: &vsiCatalogOfferingCrn,
			}
			catalogOfferingPrototype.Offering = offering
		} else {
			versionOffering := &vpcv1.CatalogOfferingVersionIdentityCatalogOfferingVersionByCRN{
				CRN: &vsiCatalogOfferingVersionCrn,
			}
			catalogOfferingPrototype.Version = versionOffering
		}
		instancePrototypeModel := &vpcv1.InstancePrototypeInstanceByCatalogOffering{
			Keys:                    []vpcv1.KeyIdentityIntf{keyIdentityModel},
			Name:                    &[]string{config.VSIName}[0],
			Profile:                 instanceProfileIdentityModel,
			VPC:                     vpcIdentityModel,
			PrimaryNetworkInterface: networkInterfacePrototypeModel,
			Zone:                    zoneIdentityModel,
		}
		instancePrototypeModel.CatalogOffering = catalogOfferingPrototype

		userDataFilePath := config.VSIUserDataFile
		userDataString := config.VSIUserDataString
		if userDataFilePath != "" {
			content, err := ioutil.ReadFile(userDataFilePath)
			if err != nil {
				err := fmt.Errorf("[ERROR] Error reading user data file. Error: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
			instancePrototypeModel.UserData = &[]string{string(content)}[0]
		} else if userDataString != "" {
			instancePrototypeModel.UserData = &[]string{string(userDataString)}[0]
		}

		if config.ResourceGroupID != "" {
			instancePrototypeModel.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{
				ID: &config.ResourceGroupID,
			}
		}

		state.Put("instance_definition", *instancePrototypeModel)

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

	} else {
		// Get Image ID
		if vsiBaseImageName != "" {
			ui.Say("Fetching ImageID...")

			options := &vpcv1.ListImagesOptions{}
			options.SetName(vsiBaseImageName)
			image, _, err := vpcService.ListImages(options)

			if err != nil {
				err := fmt.Errorf("[ERROR] Error getting image with name: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
			if image != nil && len(image.Images) == 0 {
				err := fmt.Errorf("[ERROR] Image %s not found", vsiBaseImageName)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
			vsiBaseImageID = *image.Images[0].ID
			ui.Say(fmt.Sprintf("ImageID fetched: %s", string(vsiBaseImageName)))
		}

		imageIdentityModel := &vpcv1.ImageIdentityByID{
			ID: &[]string{vsiBaseImageID}[0],
		}
		instancePrototypeModel := &vpcv1.InstancePrototypeInstanceByImage{
			Keys:                    []vpcv1.KeyIdentityIntf{keyIdentityModel},
			Name:                    &[]string{config.VSIName}[0],
			Profile:                 instanceProfileIdentityModel,
			VPC:                     vpcIdentityModel,
			Image:                   imageIdentityModel,
			PrimaryNetworkInterface: networkInterfacePrototypeModel,
			Zone:                    zoneIdentityModel,
		}

		userDataFilePath := config.VSIUserDataFile
		userDataString := config.VSIUserDataString
		if userDataFilePath != "" {
			content, err := ioutil.ReadFile(userDataFilePath)
			if err != nil {
				err := fmt.Errorf("[ERROR] Error reading user data file. Error: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
			instancePrototypeModel.UserData = &[]string{string(content)}[0]
		} else if userDataString != "" {
			instancePrototypeModel.UserData = &[]string{string(userDataString)}[0]
		}

		if config.ResourceGroupID != "" {
			instancePrototypeModel.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{
				ID: &config.ResourceGroupID,
			}
		}

		state.Put("instance_definition", *instancePrototypeModel)

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
	}

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
