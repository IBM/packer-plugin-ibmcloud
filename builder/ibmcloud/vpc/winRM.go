package vpc

import (
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

func winRMConfig(state multistep.StateBag) (*communicator.WinRMConfig, error) {
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	instanceData := state.Get("instance_data").(map[string]interface{})
	instanceID := instanceData["id"].(string)

	// Grabbing credentials for the instance
	username, password, err := client.GrabCredentials(instanceID, state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error grabbing credentials: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return nil, nil
	}
	ui.Say(fmt.Sprintf("Successfully grabbed credentials for instance (ID: %s, IP: %s)", instanceID, config.Comm.WinRMHost))

	// Configuring WinRM
	comm := communicator.WinRMConfig{
		Username: username,
		Password: password,
	}

	// Make sure to update WinRMUser and WinRMPassword in config and shared state
	config.Comm.WinRMUser = username
	config.Comm.WinRMPassword = password
	state.Put("config", config)
	state.Put("winrm_password", password)

	ui.Say(fmt.Sprintf("Attempting WinRM communication... Instance credentials (Username: %s, Password: %s)", comm.Username, comm.Password))
	return &comm, nil
}

func winRMCommHost(state multistep.StateBag) (string, error) {
	config := state.Get("config").(Config)
	return config.Comm.WinRMHost, nil
}
