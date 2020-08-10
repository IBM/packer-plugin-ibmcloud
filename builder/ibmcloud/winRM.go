package ibmcloud

import (
	"fmt"

	"github.com/hashicorp/packer/helper/communicator"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
)

func winRMConfig(state multistep.StateBag) (*communicator.WinRMConfig, error) {
	client := state.Get("client").(*SoftlayerClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	instance := state.Get("instance_data").(map[string]interface{})
	instanceID := instance["globalIdentifier"].(string)

	ui.Say(fmt.Sprintf("Grabbing credentials for instance: %s", instanceID))

	username, password, err := client.GrabCredentials(instanceID, state)

	if err != nil {
		ui.Error(err.Error())
		state.Put("error", err)
		return nil, nil
	}

	ui.Say(fmt.Sprintf("Successfully grabbed credentials for instance: %s", instanceID))

	comm := communicator.WinRMConfig{
		Username: username,
		Password: password,
	}

	ui.Say(fmt.Sprintf("Created WinRMConfig with Username: %s, Password: %s", comm.Username, comm.Password))

	// Make sure to update WinRMUser and WinRMPassword in config and shared state
	config.Comm.WinRMUser = username
	config.Comm.WinRMPassword = password
	state.Put("config", config)
	state.Put("winrm_password", password)

	return &comm, nil
}

func winRMCommHost(state multistep.StateBag) (string, error) {
	config := state.Get("config").(Config)
	return config.Comm.WinRMHost, nil
}
