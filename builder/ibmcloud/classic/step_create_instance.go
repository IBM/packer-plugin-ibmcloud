package classic

import (
	"context"
	"fmt"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCreateInstance struct {
	instanceId string
}

func (s *stepCreateInstance) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*SoftlayerClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	// The ssh_key_id can be empty if the user specified a private key
	sshKeyId := state.Get("ssh_key_id")
	var ProvisioningSshKeyId int64 = config.ProvisioningSshKeyId
	//var ProvisioningSshKeyId int64 = 767401
	if sshKeyId != nil {
		ProvisioningSshKeyId = sshKeyId.(int64)
	}

	instanceDefinition := &InstanceType{
		HostName:     config.InstanceName,
		Domain:       config.InstanceDomain,
		Datacenter:   config.DatacenterName,
		PublicVlanId: config.PublicVlanId,

		Flavor:       config.InstanceFlavor,
		Cpus:         config.InstanceCpu,
		Memory:       config.InstanceMemory,
		DiskCapacity: config.InstanceDiskCapacity,

		HourlyBillingFlag:      true,
		LocalDiskFlag:          config.InstanceLocalDiskFlag,
		PublicSecurityGroupIds: config.InstancePublicSecurityGroupIds,
		NetworkSpeed:           config.InstanceNetworkSpeed,
		ProvisioningSshKeyId:   ProvisioningSshKeyId,
		BaseImageId:            config.BaseImageId,
		BaseOsCode:             config.BaseOsCode,
	}

	ui.Say("Creating an instance...")
	instanceData, err := client.CreateInstance(*instanceDefinition)
	if err != nil {
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	state.Put("instance_data", instanceData)
	s.instanceId = instanceData["globalIdentifier"].(string)
	ui.Say(fmt.Sprintf("Created instance, id: '%s'", instanceData["globalIdentifier"].(string)))

	return multistep.ActionContinue
}

func (s *stepCreateInstance) Cleanup(state multistep.StateBag) {
	client := state.Get("client").(*SoftlayerClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	if s.instanceId == "" {
		return
	}

	ui.Say("Waiting for the instance to have no active transactions before destroying it...")

	// We should wait until the instance is up/have no transactions,
	// since if the instance will have some assigned transactions the destroy API call will fail
	err := client.waitForInstanceReady(s.instanceId, config.StateTimeout)
	if err != nil {
		log.Printf("Error destroying instance: %v", err.Error())
		ui.Error(fmt.Sprintf("Error waiting for instance to become ACTIVE for instance (%s)", s.instanceId))
	}

	ui.Say("Destroying instance...")
	err = client.DestroyInstance(s.instanceId)
	if err != nil {
		log.Printf("Error destroying instance: %v", err.Error())
		ui.Error(fmt.Sprintf("Error cleaning up the instance. Please delete the instance (%s) manually", s.instanceId))
	}
}
