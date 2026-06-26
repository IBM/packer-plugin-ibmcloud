package vpc

import (
	"context"
	"fmt"
	"os"
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
	vsiBaseImageID := state.Get("baseImageID").(string)
	vsiCatalogOfferingCrn := config.CatalogOfferingCRN
	vsiCatalogOfferingVersionCrn := config.CatalogOfferingVersionCRN
	vsiBootVolumeID := config.VSIBootVolumeID
	vsiBootSnapshotId := config.VSIBootSnapshotID

	vsiCapacity := config.VSIBootCapacity

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
		if int64(vsiCapacity) != 0 {
			instancePrototypeModel.BootVolumeAttachment = &vpcv1.VolumeAttachmentPrototypeInstanceByImageContext{
				Volume: bootVolumePrototype(&config),
			}
		}
		instancePrototypeModel.VolumeAttachments = dataVolumeAttachments(&config)
		instancePrototypeModel.CatalogOffering = catalogOfferingPrototype

		userDataFilePath := config.VSIUserDataFile
		userDataString := config.VSIUserDataString
		if userDataFilePath != "" {
			content, err := os.ReadFile(userDataFilePath)
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
		} else if config.ResourceGroupName != "" {
			derivedResourceGroupId := state.Get("derived_resource_group_id")
			if derivedResourceGroupId != nil && derivedResourceGroupId.(string) != "" {
				derivedResourceGroupIdStr := derivedResourceGroupId.(string)
				instancePrototypeModel.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{
					ID: &derivedResourceGroupIdStr,
				}
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

	} else if vsiBaseImageName != "" || vsiBaseImageID != "" {

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
		if int64(vsiCapacity) != 0 {
			instancePrototypeModel.BootVolumeAttachment = &vpcv1.VolumeAttachmentPrototypeInstanceByImageContext{
				Volume: bootVolumePrototype(&config),
			}
		}
		instancePrototypeModel.VolumeAttachments = dataVolumeAttachments(&config)

		userDataFilePath := config.VSIUserDataFile
		userDataString := config.VSIUserDataString
		if userDataFilePath != "" {
			content, err := os.ReadFile(userDataFilePath)
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
		} else if config.ResourceGroupName != "" {
			derivedResourceGroupId := state.Get("derived_resource_group_id")
			if derivedResourceGroupId != nil && derivedResourceGroupId.(string) != "" {
				derivedResourceGroupIdStr := derivedResourceGroupId.(string)
				instancePrototypeModel.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{
					ID: &derivedResourceGroupIdStr,
				}
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
	} else if vsiBootVolumeID != "" {
		ui.Say("Creating instance with boot volume ID")
		volumeIdentity := &vpcv1.VolumeIdentity{
			ID: &vsiBootVolumeID,
		}
		bootVolumeAttachment := &vpcv1.VolumeAttachmentPrototypeInstanceByVolumeContext{
			Volume: volumeIdentity,
		}
		instancePrototypeModel := &vpcv1.InstancePrototypeInstanceByVolume{
			Keys:                    []vpcv1.KeyIdentityIntf{keyIdentityModel},
			Name:                    &[]string{config.VSIName}[0],
			Profile:                 instanceProfileIdentityModel,
			VPC:                     vpcIdentityModel,
			BootVolumeAttachment:    bootVolumeAttachment,
			PrimaryNetworkInterface: networkInterfacePrototypeModel,
			Zone:                    zoneIdentityModel,
		}
		instancePrototypeModel.VolumeAttachments = dataVolumeAttachments(&config)

		userDataFilePath := config.VSIUserDataFile
		userDataString := config.VSIUserDataString
		if userDataFilePath != "" {
			content, err := os.ReadFile(userDataFilePath)
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
		} else if config.ResourceGroupName != "" {
			derivedResourceGroupId := state.Get("derived_resource_group_id")
			if derivedResourceGroupId != nil && derivedResourceGroupId.(string) != "" {
				derivedResourceGroupIdStr := derivedResourceGroupId.(string)
				instancePrototypeModel.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{
					ID: &derivedResourceGroupIdStr,
				}
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

		ui.Say("Instance successfully created with the provided boot volume!")
		ui.Say(fmt.Sprintf("Instance's Name: %s", *instanceData.Name))
		ui.Say(fmt.Sprintf("Instance's ID: %s", *instanceData.ID))
	} else if vsiBootSnapshotId != "" {
		ui.Say("Creating instance with boot snapshot ID")
		sourceSnapshot := &vpcv1.SnapshotIdentity{
			ID: &vsiBootSnapshotId,
		}
		bootVolumeAttachment := &vpcv1.VolumeAttachmentPrototypeInstanceBySourceSnapshotContext{
			Volume: snapshotBootVolumePrototype(&config, sourceSnapshot),
		}
		instancePrototypeModel := &vpcv1.InstancePrototypeInstanceBySourceSnapshot{
			Keys:                    []vpcv1.KeyIdentityIntf{keyIdentityModel},
			Name:                    &[]string{config.VSIName}[0],
			Profile:                 instanceProfileIdentityModel,
			VPC:                     vpcIdentityModel,
			BootVolumeAttachment:    bootVolumeAttachment,
			PrimaryNetworkInterface: networkInterfacePrototypeModel,
			Zone:                    zoneIdentityModel,
		}
		instancePrototypeModel.VolumeAttachments = dataVolumeAttachments(&config)

		userDataFilePath := config.VSIUserDataFile
		userDataString := config.VSIUserDataString
		if userDataFilePath != "" {
			content, err := os.ReadFile(userDataFilePath)
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

		ui.Say("Instance successfully created with the provided boot snapshot!")
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
		if state.Get("floating_ip") != nil && state.Get("floating_ip_id") != nil {
			floatingIP := state.Get("floating_ip").(string)
			ui.Say(fmt.Sprintf("Releasing the Floating IP: %s ...", floatingIP))

			floatingIPID := state.Get("floating_ip_id").(string)

			options := vpcService.NewGetFloatingIPOptions(floatingIPID)
			floatingIPresponse, response, err := vpcService.GetFloatingIP(options)
			if err != nil && response.StatusCode != 404 {
				err := fmt.Errorf("[ERROR] Error getting the Floating IP: %s", err)
				state.Put("error", err)
				ui.Error(err.Error())
				// log.Fatalf(err.Error())
				return
			}
			// Only proceed if the Floating IP still exists (not 404)
			if response.StatusCode != 404 && floatingIPresponse.Status != nil {
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
			} else if response.StatusCode == 404 {
				ui.Say("The Floating IP was already deleted or does not exist.")
			}
		}
	}

	// Wait a couple of seconds before attempting to delete the instance.
	time.Sleep(2 * time.Second)

	// Check if instance_data exists in state before attempting deletion
	if state.Get("instance_data") != nil {
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
					ui.Say("Instance deleted Successfully")
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
	}

	// Deleting Security Group's rule
	if state.Get("security_group_rule_id") != nil && state.Get("security_group_id") != nil {
		ruleID := state.Get("security_group_rule_id").(string)
		securityGroupID := state.Get("security_group_id").(string)
		ui.Say(fmt.Sprintf("Deleting Security Group's rule %s ...", ruleID))
		sgRuleOptions := &vpcv1.DeleteSecurityGroupRuleOptions{}
		sgRuleOptions.SetSecurityGroupID(securityGroupID)
		sgRuleOptions.SetID(ruleID)
		sgRuleResponse, sgRuleErr := vpcService.DeleteSecurityGroupRule(sgRuleOptions)

		if sgRuleErr != nil {
			// Check if it's a 404 (resource already deleted)
			if sgRuleResponse != nil && sgRuleResponse.StatusCode == 404 {
				ui.Say("The Security Group's rule was already deleted or does not exist.")
			} else {
				sgRuleErr := fmt.Errorf("[ERROR] Error deleting Security Group's rule %s. Please delete it manually: %s", ruleID, sgRuleErr)
				state.Put("error", sgRuleErr)
				ui.Error(sgRuleErr.Error())
				// log.Fatalf(err.Error())
				return
			}
		} else if sgRuleResponse.StatusCode == 204 {
			ui.Say("The Security Group's rule was successfully deleted!")
		}
	}

	// Wait a couple of seconds before attempting to delete the security group.
	time.Sleep(10 * time.Second)

	// Deleting Security Group (only if we created it, not if user provided one)
	if config.SecurityGroupID == "" {
		if state.Get("security_group_name") != nil && state.Get("security_group_id") != nil {
			securityGroupName := state.Get("security_group_name").(string)
			securityGroupID := state.Get("security_group_id").(string)
			ui.Say(fmt.Sprintf("Deleting Security Group %s ...", securityGroupName))
			sgOptions := &vpcv1.DeleteSecurityGroupOptions{}
			sgOptions.SetID(securityGroupID)
			sgResponse, err := vpcService.DeleteSecurityGroup(sgOptions)
			if err != nil {
				// Check if it's a 404 (resource already deleted)
				if sgResponse != nil && sgResponse.StatusCode == 404 {
					ui.Say("The Security Group was already deleted or does not exist.")
				} else {
					err := fmt.Errorf("[ERROR] Error deleting Security Group %s. Please delete it manually: %s", securityGroupName, err)
					state.Put("error", err)
					ui.Error(err.Error())
					// log.Fatalf(err.Error())
					return
				}
			} else if sgResponse.StatusCode == 204 {
				ui.Say("The Security Group was successfully deleted!")
			}
		}
	}

}

func bootVolumePrototype(config *Config) *vpcv1.VolumePrototypeInstanceByImageContext {
	capacity := int64(config.VSIBootCapacity)
	profile := "general-purpose"
	if config.VSIBootProfile != "" {
		profile = config.VSIBootProfile
	}
	vol := &vpcv1.VolumePrototypeInstanceByImageContext{
		Capacity: &capacity,
		Profile:  &vpcv1.VolumeProfileIdentity{Name: &profile},
	}
	// iops/bandwidth are passed through whenever set; Config.Prepare is the gate
	// that restricts them to the custom/sdp profiles IBM honors them on.
	if config.VSIBootIops != 0 {
		iops := int64(config.VSIBootIops)
		vol.Iops = &iops
	}
	if config.VSIBootBandwidth != 0 {
		bandwidth := int64(config.VSIBootBandwidth)
		vol.Bandwidth = &bandwidth
	}
	return vol
}

// dataVolumeAttachments builds the data-volume attachments for the builder VSI,
// or nil when no data volume is configured. The volume is created with the
// instance and DeleteVolumeOnInstanceDelete=true, so it is deleted together with
// the builder VSI when the instance is torn down. It is never part of the
// captured image (capture is taken from the boot volume only), so a build can
// keep large transient writes — build caches, downloads, from-source build
// trees — off the boot volume and out of the exported image. Call this from
// every create path so the data volume is attached regardless of how the builder
// VSI is sourced.
func dataVolumeAttachments(config *Config) []vpcv1.VolumeAttachmentPrototype {
	if config.VSIDataCapacity == 0 {
		return nil
	}
	capacity := int64(config.VSIDataCapacity)
	profile := "general-purpose"
	if config.VSIDataProfile != "" {
		profile = config.VSIDataProfile
	}
	vol := &vpcv1.VolumeAttachmentPrototypeVolumeVolumePrototypeInstanceContext{
		Capacity: &capacity,
		Profile:  &vpcv1.VolumeProfileIdentity{Name: &profile},
	}
	// iops/bandwidth are passed through whenever set; Config.Prepare is the gate
	// that restricts them to the custom/sdp profiles IBM honors them on.
	if config.VSIDataIops != 0 {
		iops := int64(config.VSIDataIops)
		vol.Iops = &iops
	}
	if config.VSIDataBandwidth != 0 {
		bandwidth := int64(config.VSIDataBandwidth)
		vol.Bandwidth = &bandwidth
	}
	deleteWithInstance := true
	return []vpcv1.VolumeAttachmentPrototype{{
		DeleteVolumeOnInstanceDelete: &deleteWithInstance,
		Volume:                       vol,
	}}
}

// snapshotBootVolumePrototype builds the boot volume for the
// create-from-snapshot path. It mirrors bootVolumePrototype but for the
// snapshot SDK type, which is why the helper cannot be shared. Unlike the
// by-image path, capacity is optional here: when vsi_boot_vol_capacity is unset
// the restored volume inherits the snapshot's size, so we only set it when the
// user asked for a specific capacity.
func snapshotBootVolumePrototype(config *Config, sourceSnapshot vpcv1.SnapshotIdentityIntf) *vpcv1.VolumePrototypeInstanceBySourceSnapshotContext {
	profile := "general-purpose"
	if config.VSIBootProfile != "" {
		profile = config.VSIBootProfile
	}
	vol := &vpcv1.VolumePrototypeInstanceBySourceSnapshotContext{
		Profile:        &vpcv1.VolumeProfileIdentity{Name: &profile},
		SourceSnapshot: sourceSnapshot,
	}
	if config.VSIBootCapacity != 0 {
		capacity := int64(config.VSIBootCapacity)
		vol.Capacity = &capacity
	}
	// iops/bandwidth are passed through whenever set; Config.Prepare is the gate
	// that restricts them to the custom/sdp profiles IBM honors them on.
	if config.VSIBootIops != 0 {
		iops := int64(config.VSIBootIops)
		vol.Iops = &iops
	}
	if config.VSIBootBandwidth != 0 {
		bandwidth := int64(config.VSIBootBandwidth)
		vol.Bandwidth = &bandwidth
	}
	return vol
}
