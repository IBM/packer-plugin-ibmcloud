package vpc

import (
	"context"
	"fmt"
	"log"
	"regexp"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepCaptureImage struct{}

func (s *stepCaptureImage) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	instanceData := state.Get("instance_data").(*vpcv1.Instance)
	instanceID := *instanceData.ID

	ui.Say(fmt.Sprintf("Stopping instance ID: %s ...", instanceID))
	status, err := client.manageInstance(instanceID, "stop", state)
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
	bootVolumeAttachment := instanceData.BootVolumeAttachment
	bootVolume := bootVolumeAttachment.Volume
	bootVolumeId := *bootVolume.ID

	validName := regexp.MustCompile(`[^a-z0-9\-]+`)

	config.ImageName = validName.ReplaceAllString(config.ImageName, "")

	options := &vpcv1.CreateImageOptions{}
	imagePrototype := &vpcv1.ImagePrototypeImageBySourceVolume{
		Name: &config.ImageName,
		SourceVolume: &vpcv1.VolumeIdentityByID{
			ID: &bootVolumeId,
		},
	}
	if config.ResourceGroupID != "" {
		imagePrototype.ResourceGroup = &vpcv1.ResourceGroupIdentityByID{
			ID: &config.ResourceGroupID,
		}
	}

	options.SetImagePrototype(imagePrototype)

	imageData, _, err := vpcService.CreateImage(options)

	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that creates the image. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return multistep.ActionHalt
	}

	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating the Image: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}

	imageId := *imageData.ID
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
