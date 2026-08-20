package vpc

import (
	"context"
	"errors"
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
	client := state.Get("client").(*IBMCloudClient)

	// bake_subnets is the shuffled subnet/zone list from stepGetSubnetInfo. The
	// builder VSI's profile can intermittently have no host capacity in a given
	// zone (a capacity status reason; see capacityStatusReasonCodes), so try each
	// subnet in turn: create the VSI, wait for it to start, and on a capacity
	// failure delete it and move to the next zone.
	subnets := state.Get("bake_subnets").([]subnetZone)

	for i, sn := range subnets {
		if len(subnets) > 1 {
			ui.Say(fmt.Sprintf("Creating Instance in subnet %s (zone %s) [attempt %d/%d]...", sn.ID, sn.Zone, i+1, len(subnets)))
		} else {
			ui.Say("Creating Instance...")
		}

		instanceData, err := step.createInstance(state, sn.ID, sn.Zone)
		if err != nil {
			// A create-time error is not the capacity case, so it is fatal rather
			// than retried in another zone.
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		// Record the instance immediately so Cleanup can delete it if the wait
		// below fails on the final attempt.
		state.Put("instance_data", instanceData)
		ui.Say(fmt.Sprintf("Instance created: %s (%s). Waiting for it to start...", *instanceData.Name, *instanceData.ID))

		waitErr := client.waitForResourceReady(*instanceData.ID, "instances", config.StateTimeout, state)
		if waitErr == nil {
			ui.Say("Instance successfully started!")
			return multistep.ActionContinue
		}

		// Only a capacity/host-placement failure is worth trying another zone; any
		// other failure (and the last subnet) is fatal.
		var capErr *capacityError
		if !errors.As(waitErr, &capErr) || i == len(subnets)-1 {
			state.Put("error", waitErr)
			ui.Error(waitErr.Error())
			return multistep.ActionHalt
		}

		ui.Say(fmt.Sprintf("Zone %s could not start the instance (%s). Trying the next subnet...", sn.Zone, waitErr))
		// Delete the failed VSI before the next attempt, then clear instance_data
		// so Cleanup does not try to delete an instance that is already gone.
		if delErr := deleteInstanceAndWait(vpcService(state), ui, *instanceData.ID, config.StateTimeout); delErr != nil {
			state.Put("error", delErr)
			ui.Error(delErr.Error())
			return multistep.ActionHalt
		}
		state.Put("instance_data", nil)
	}

	// Unreachable: Config.Prepare guarantees at least one subnet.
	err := fmt.Errorf("[ERROR] no subnets were available to create the builder instance")
	state.Put("error", err)
	ui.Error(err.Error())
	return multistep.ActionHalt
}

// createInstance builds the instance prototype for the configured source
// (catalog offering, base image, boot volume, or boot snapshot) in the given
// subnet/zone and creates it. It returns the created instance or an error; it
// does not wait for the instance to start or mutate build state beyond recording
// the instance definition.
func (step *stepCreateInstance) createInstance(state multistep.StateBag, subnetID, zone string) (*vpcv1.Instance, error) {
	config := state.Get("config").(Config)
	svc := vpcService(state)

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
		ID: &[]string{subnetID}[0],
	}

	// Create VirtualNetworkInterface for the new PrimaryNetworkAttachment
	virtualNetworkInterfacePrototype := &vpcv1.InstanceNetworkAttachmentPrototypeVirtualNetworkInterface{
		Subnet: subnetIdentityModel,
	}

	// Create PrimaryNetworkAttachment
	primaryNetworkAttachment := &vpcv1.InstanceNetworkAttachmentPrototype{
		VirtualNetworkInterface: virtualNetworkInterfacePrototype,
	}

	zoneIdentityModel := &vpcv1.ZoneIdentityByName{
		Name: &[]string{zone}[0],
	}

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
			Keys:                     []vpcv1.KeyIdentityIntf{keyIdentityModel},
			Name:                     &[]string{config.VSIName}[0],
			Profile:                  instanceProfileIdentityModel,
			VPC:                      vpcIdentityModel,
			PrimaryNetworkAttachment: primaryNetworkAttachment,
			Zone:                     zoneIdentityModel,
		}
		if int64(vsiCapacity) != 0 {
			instancePrototypeModel.BootVolumeAttachment = &vpcv1.VolumeAttachmentPrototypeInstanceByImageContext{
				Volume: bootVolumePrototype(&config),
			}
		}
		instancePrototypeModel.VolumeAttachments = dataVolumeAttachments(&config)
		instancePrototypeModel.CatalogOffering = catalogOfferingPrototype

		if err := applyUserData(&config, &instancePrototypeModel.UserData); err != nil {
			return nil, err
		}
		instancePrototypeModel.ResourceGroup = resourceGroupIdentity(&config, state)

		state.Put("instance_definition", *instancePrototypeModel)
		return doCreate(svc, instancePrototypeModel)
	}

	if vsiBaseImageName != "" || vsiBaseImageID != "" {
		imageIdentityModel := &vpcv1.ImageIdentityByID{
			ID: &[]string{vsiBaseImageID}[0],
		}
		instancePrototypeModel := &vpcv1.InstancePrototypeInstanceByImage{
			Keys:                     []vpcv1.KeyIdentityIntf{keyIdentityModel},
			Name:                     &[]string{config.VSIName}[0],
			Profile:                  instanceProfileIdentityModel,
			VPC:                      vpcIdentityModel,
			Image:                    imageIdentityModel,
			PrimaryNetworkAttachment: primaryNetworkAttachment,
			Zone:                     zoneIdentityModel,
		}
		if int64(vsiCapacity) != 0 {
			instancePrototypeModel.BootVolumeAttachment = &vpcv1.VolumeAttachmentPrototypeInstanceByImageContext{
				Volume: bootVolumePrototype(&config),
			}
		}
		instancePrototypeModel.VolumeAttachments = dataVolumeAttachments(&config)

		if err := applyUserData(&config, &instancePrototypeModel.UserData); err != nil {
			return nil, err
		}
		instancePrototypeModel.ResourceGroup = resourceGroupIdentity(&config, state)

		state.Put("instance_definition", *instancePrototypeModel)
		return doCreate(svc, instancePrototypeModel)
	}

	if vsiBootVolumeID != "" {
		volumeIdentity := &vpcv1.VolumeIdentity{
			ID: &vsiBootVolumeID,
		}
		bootVolumeAttachment := &vpcv1.VolumeAttachmentPrototypeInstanceByVolumeContext{
			Volume: volumeIdentity,
		}
		instancePrototypeModel := &vpcv1.InstancePrototypeInstanceByVolume{
			Keys:                     []vpcv1.KeyIdentityIntf{keyIdentityModel},
			Name:                     &[]string{config.VSIName}[0],
			Profile:                  instanceProfileIdentityModel,
			VPC:                      vpcIdentityModel,
			BootVolumeAttachment:     bootVolumeAttachment,
			PrimaryNetworkAttachment: primaryNetworkAttachment,
			Zone:                     zoneIdentityModel,
		}
		instancePrototypeModel.VolumeAttachments = dataVolumeAttachments(&config)

		if err := applyUserData(&config, &instancePrototypeModel.UserData); err != nil {
			return nil, err
		}
		instancePrototypeModel.ResourceGroup = resourceGroupIdentity(&config, state)

		state.Put("instance_definition", *instancePrototypeModel)
		return doCreate(svc, instancePrototypeModel)
	}

	if vsiBootSnapshotId != "" {
		sourceSnapshot := &vpcv1.SnapshotIdentity{
			ID: &vsiBootSnapshotId,
		}
		bootVolumeAttachment := &vpcv1.VolumeAttachmentPrototypeInstanceBySourceSnapshotContext{
			Volume: snapshotBootVolumePrototype(&config, sourceSnapshot),
		}
		instancePrototypeModel := &vpcv1.InstancePrototypeInstanceBySourceSnapshot{
			Keys:                     []vpcv1.KeyIdentityIntf{keyIdentityModel},
			Name:                     &[]string{config.VSIName}[0],
			Profile:                  instanceProfileIdentityModel,
			VPC:                      vpcIdentityModel,
			BootVolumeAttachment:     bootVolumeAttachment,
			PrimaryNetworkAttachment: primaryNetworkAttachment,
			Zone:                     zoneIdentityModel,
		}
		instancePrototypeModel.VolumeAttachments = dataVolumeAttachments(&config)

		if err := applyUserData(&config, &instancePrototypeModel.UserData); err != nil {
			return nil, err
		}
		// The snapshot path only supports resource_group_id (no
		// resource_group_name derivation, unlike the other source paths).
		if config.ResourceGroupID != "" {
			instancePrototypeModel.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{
				ID: &config.ResourceGroupID,
			}
		}

		state.Put("instance_definition", *instancePrototypeModel)
		return doCreate(svc, instancePrototypeModel)
	}

	return nil, fmt.Errorf("[ERROR] no instance source configured")
}

func (step *stepCreateInstance) Cleanup(state multistep.StateBag) {
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)
	svc := vpcService(state)

	// Delete Floating IP if it was created (VSI Interface was set as public)
	if config.VSIInterface == "public" {
		if state.Get("floating_ip") != nil && state.Get("floating_ip_id") != nil {
			floatingIP := state.Get("floating_ip").(string)
			ui.Say(fmt.Sprintf("Releasing the Floating IP: %s ...", floatingIP))

			floatingIPID := state.Get("floating_ip_id").(string)

			options := svc.NewGetFloatingIPOptions(floatingIPID)
			floatingIPresponse, response, err := svc.GetFloatingIP(options)
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
					options := svc.NewDeleteFloatingIPOptions(floatingIPID)
					result, err := svc.DeleteFloatingIP(options)

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

	// A capacity-fallback attempt clears instance_data after deleting its failed
	// VSI, so this only fires for the instance still standing at cleanup.
	if state.Get("instance_data") != nil {
		instanceData := state.Get("instance_data").(*vpcv1.Instance)
		if err := deleteInstanceAndWait(svc, ui, *instanceData.ID, config.StateTimeout); err != nil {
			state.Put("error", err)
			ui.Error(err.Error())
			return
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
		sgRuleResponse, sgRuleErr := svc.DeleteSecurityGroupRule(sgRuleOptions)

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
			sgResponse, err := svc.DeleteSecurityGroup(sgOptions)
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

// vpcService returns the shared vpcv1 client from build state, or nil if it has
// not been created yet.
func vpcService(state multistep.StateBag) *vpcv1.VpcV1 {
	if svc := state.Get("vpcService"); svc != nil {
		return svc.(*vpcv1.VpcV1)
	}
	return nil
}

// doCreate issues the CreateInstance call for a built prototype, wrapping the
// error consistently across the create paths.
func doCreate(svc *vpcv1.VpcV1, prototype vpcv1.InstancePrototypeIntf) (*vpcv1.Instance, error) {
	instanceData, _, err := svc.CreateInstance(svc.NewCreateInstanceOptions(prototype))
	if err != nil {
		return nil, fmt.Errorf("[ERROR] Error creating the instance: %s", err)
	}
	return instanceData, nil
}

// applyUserData sets *dst from vsi_user_data_file or vsi_user_data (mutually
// exclusive per Config.Prepare). It is a no-op when neither is configured.
func applyUserData(config *Config, dst **string) error {
	if config.VSIUserDataFile != "" {
		content, err := os.ReadFile(config.VSIUserDataFile)
		if err != nil {
			return fmt.Errorf("[ERROR] Error reading user data file. Error: %s", err)
		}
		*dst = &[]string{string(content)}[0]
		return nil
	}
	if config.VSIUserDataString != "" {
		*dst = &[]string{config.VSIUserDataString}[0]
	}
	return nil
}

// resourceGroupIdentity resolves the resource group for the instance from
// resource_group_id, or from the id derived from resource_group_name in
// stepVerifyInput. Returns nil when neither is configured (the account default
// resource group is then used).
func resourceGroupIdentity(config *Config, state multistep.StateBag) vpcv1.ResourceGroupIdentityIntf {
	if config.ResourceGroupID != "" {
		return &vpcv1.ResourceGroupIdentityByID{ID: &config.ResourceGroupID}
	}
	if config.ResourceGroupName != "" {
		if derived := state.Get("derived_resource_group_id"); derived != nil && derived.(string) != "" {
			id := derived.(string)
			return &vpcv1.ResourceGroupIdentityByID{ID: &id}
		}
	}
	return nil
}

// deleteInstanceAndWait deletes the instance and blocks until the API reports it
// gone (404), bounded by timeout. It is used both to tear down the successful
// builder VSI at cleanup and to remove a VSI that failed to start before the
// capacity fallback tries the next subnet. A transient status-check failure is
// tolerated and retried until the deadline rather than aborting the delete —
// which, on the fallback path, would abort the whole build over one API blip.
func deleteInstanceAndWait(svc *vpcv1.VpcV1, ui packer.Ui, instanceID string, timeout time.Duration) error {
	ui.Say(fmt.Sprintf("Deleting Instance ID: %s ...", instanceID))
	options := &vpcv1.DeleteInstanceOptions{}
	options.SetID(instanceID)
	if _, err := svc.DeleteInstance(options); err != nil {
		return fmt.Errorf("[ERROR] Error deleting the instance. Please delete it manually: %s", err)
	}
	deadline := time.Now().Add(timeout)
	for {
		getOptions := &vpcv1.GetInstanceOptions{}
		getOptions.SetID(instanceID)
		instance, response, err := svc.GetInstance(getOptions)
		if err != nil {
			if response != nil && response.StatusCode == 404 {
				ui.Say("Instance deleted Successfully")
				return nil
			}
			ui.Say(fmt.Sprintf("Instance status check failed, retrying: %s", err))
		} else if instance != nil {
			ui.Say(fmt.Sprintf("Instance status :-  %s", *instance.Status))
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("[ERROR] Timed out waiting for instance %s to delete. Please verify it was removed manually.", instanceID)
		}
		time.Sleep(defaultPollInterval)
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
	// that restricts iops to the custom/sdp profiles and bandwidth to sdp, the
	// profiles IBM honors them on.
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
	// that restricts iops to the custom/sdp profiles and bandwidth to sdp, the
	// profiles IBM honors them on.
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
	// that restricts iops to the custom/sdp profiles and bandwidth to sdp, the
	// profiles IBM honors them on.
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
