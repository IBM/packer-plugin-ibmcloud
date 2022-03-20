package vpc

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepGetIP struct{}

func (step *stepGetIP) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	instanceData := state.Get("instance_data").(map[string]interface{})

	ui.Say(fmt.Sprintf("Getting %s IP...", strings.Title(config.VSIInterface)))
	var ipAddress string
	if config.VSIInterface == "private" {
		primaryNetworkInterface := instanceData["primary_network_interface"].(map[string]interface{})
		// ipAddress = primaryNetworkInterface["primary_ipv4_address"].(string)

		// Post 3/29/22 Reserved IP P2
		ipAddress = primaryNetworkInterface["primary_ip"].(string)

	} else if config.VSIInterface == "public" {
		ui.Say("Reserve a Floating IP and associate it to the instance's network interface")

		// Create Floating IP
		ui.Say("Reserving a Floating IP")
		floatingIPData, errIP := client.createFloatingIP(state)
		if errIP != nil {
			err := fmt.Errorf("[ERROR] Error creating FloatingIP: %s", errIP)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}

		// Wait until the Floating IP is ACTIVE
		ui.Say("Waiting for the Floating IP to become ACTIVE...")
		floatingIPID := *floatingIPData.ID
		state.Put("floating_ip_id", floatingIPID)

		err := client.waitForResourceReady(floatingIPID, "floating_ips", config.StateTimeout, state)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error waiting for Floating IP to become ACTIVE: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
		ui.Say("Floating IP is ACTIVE!")
		ipAddress = *floatingIPData.Address
	}

	ui.Say(fmt.Sprintf("%s IP Address: %s", strings.Title(config.VSIInterface), ipAddress))
	state.Put("floating_ip", ipAddress)

	///// Update the Communicator with the ipAddres value /////
	if config.Comm.Type == "winrm" {
		config.Comm.WinRMHost = ipAddress
	} else if config.Comm.Type == "ssh" {
		config.Comm.SSHHost = ipAddress
	}
	state.Put("config", config)

	// Write IP Address to ANSIBLE_INVENTORY_FILE, so there is no need to
	// manually accept the connection with the instance during SSH Communication
	hostsFilePath := os.Getenv("ANSIBLE_INVENTORY_FILE")
	if hostsFilePath == "" {
		// No inventory file specified, continuing on to next step
		return multistep.ActionContinue
	}

	// ui.Say(fmt.Sprintf("Writing IP address to file %s", hostsFilePath))
	ipAddressBytes := []byte(fmt.Sprintf("%s\n", ipAddress))
	err := ioutil.WriteFile(hostsFilePath, ipAddressBytes, 0644)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed to write IP address to file: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}
	// ui.Say(fmt.Sprintf("IP address has been written into file %s", hostsFilePath))
	return multistep.ActionContinue
}

func (client *stepGetIP) Cleanup(state multistep.StateBag) {}
