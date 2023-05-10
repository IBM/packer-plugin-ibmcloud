package vpc

import (
	"context"
	"fmt"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type StepImageExport struct{}

func (step *StepImageExport) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	imageExportJobName := config.ImageExportJobName
	storageBucketName := config.StorageBucketName
	storageBucketCRN := config.StorageBucketCRN
	format := config.Format

	ui.Say("Creating Image Export Job...")
	createImageExportJobOptions := &vpcv1.CreateImageExportJobOptions{}
	createImageExportJobOptions.SetImageID(config.ImageID)

	storageBucket := &vpcv1.CloudObjectStorageBucketIdentity{}
	if storageBucketName != "" {
		ui.Say(fmt.Sprintf("Exporting image %v to destination: %v", config.ImageID, storageBucketName))
		storageBucket.Name = &storageBucketName
	} else {
		ui.Say(fmt.Sprintf("Exporting image %v to destination: %v", config.ImageID, storageBucketCRN))
		storageBucket.CRN = &storageBucketCRN
	}
	createImageExportJobOptions.SetStorageBucket(storageBucket)
	createImageExportJobOptions.SetFormat(format)
	createImageExportJobOptions.SetName(imageExportJobName)

	imageExportJob, _, err := vpcService.CreateImageExportJob(createImageExportJobOptions)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating image export job: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	jobId := *imageExportJob.ID
	state.Put("image_id", config.ImageID)
	state.Put("image_export_job_id", jobId)
	ui.Say("Image export job created successfully!") // Image exported job created successfully
	ui.Say(fmt.Sprintf("Image Export Job's ID: %s", *imageExportJob.ID))
	return multistep.ActionContinue
}

func (step *StepImageExport) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	ui.Say("")
	ui.Say("*******************************************************************************************************")
	ui.Say("* Successfully Executed IBM Cloud Packer Post Processor Plugin for Image Export  - VPC Infrastructure *")
	ui.Say("*******************************************************************************************************")
}
