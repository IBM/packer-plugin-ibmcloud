package vpc

import (
	"context"
	"fmt"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCaptureImage struct{}

func (s *stepCaptureImage) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	instanceData := state.Get("instance_data").(map[string]interface{})
	instanceID := instanceData["id"].(string)

	ui.Say(fmt.Sprintf("Stopping instance ID: %s ...", instanceID))
	status, err := client.manageInstance(instanceID, "instances", "stop", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error stopping the instance: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	if status != "stopped" {
		err := client.waitForResourceDown(instanceID, "instances", config.StateTimeout, state)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error stopping the instance: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
	}
	ui.Say("Instance successfully stopped!")

	ui.Say(fmt.Sprintf("Creating an Image from instance ID: %s ...", instanceID))
	bootVolumeAttachment := instanceData["boot_volume_attachment"].(map[string]interface{})
	bootVolume := bootVolumeAttachment["volume"].(map[string]interface{})
	bootVolumeId := bootVolume["id"].(string)
	// ui.Say(fmt.Sprintf("Instance's Boot-Volume-ID: %s", bootVolumeId))

	imageRequest := &ImageReq{
		Name: config.ImageName,
		SourceVolume: &ResourceByID{
			Id: bootVolumeId,
		},
	}

	if config.ResourceGroupID != "" {
		imageRequest.ResourceGroup = &ResourceByID{
			Id: config.ResourceGroupID,
		}
	}

	imageData, err := client.createImage(state, *imageRequest)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating the Image: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	imageId := imageData["id"].(string)
	state.Put("image_id", imageId)

	ui.Say("Image Successfully created!")
	ui.Say(fmt.Sprintf("Image's Name: %s", config.ImageName))
	ui.Say(fmt.Sprintf("Image's ID: %s", imageId))

	ui.Say("Waiting for the Image to become AVAILABLE...")
	err2 := client.waitForResourceReady(imageId, "images", config.StateTimeout, state)
	if err2 != nil {
		err := fmt.Errorf("[ERROR] Error waiting for the Image to become AVAILABLE: %s", err2)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}
	ui.Say("Image is now AVAILABLE!")
	return multistep.ActionContinue
}

func (s *stepCaptureImage) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	ui.Say("")
	ui.Say("****************************************************************************")
	ui.Say("* Cleaning Up all temporary infrastructure created during packer execution *")
	ui.Say("****************************************************************************")
	ui.Say("")
}
