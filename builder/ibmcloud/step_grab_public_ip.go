package ibmcloud

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

type stepGrabPublicIP struct{}

func (self *stepGrabPublicIP) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*SoftlayerClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	instance := state.Get("instance_data").(map[string]interface{})
	instanceID := instance["globalIdentifier"].(string)

	ipAddress, err := client.getInstancePublicIp(instanceID)
	if err != nil {
		err := fmt.Errorf("Failed to fetch Public IP address for instance '%s'", instanceID)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Grabbed IP Address: %s", ipAddress))

	if config.Comm.Type == "winrm" {
		config.Comm.WinRMHost = ipAddress
	} else if config.Comm.Type == "ssh" {
		config.Comm.SSHHost = ipAddress
	}
	state.Put("config", config)

	hostsFilePath := os.Getenv("ANSIBLE_INVENTORY_FILE")
	if hostsFilePath == "" {
		// No inventory file specified, continuing on to next step
		return multistep.ActionContinue
	}

	ui.Say(fmt.Sprintf("Writing ip address to file %s", hostsFilePath))

	ipAddressBytes := []byte(fmt.Sprintf("%s\n", ipAddress))

	err = ioutil.WriteFile(hostsFilePath, ipAddressBytes, 0644)
	if err != nil {
		err := fmt.Errorf("Failed to write ip address to file %s", hostsFilePath)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	ui.Say(fmt.Sprintf("Wrote ip address to file %s", hostsFilePath))

	return multistep.ActionContinue
}

func (client *stepGrabPublicIP) Cleanup(state multistep.StateBag) {}
