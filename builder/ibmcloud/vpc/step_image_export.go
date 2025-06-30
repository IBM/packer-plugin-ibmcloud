package vpc

import (
	"context"
	"fmt"
	"log"
	"time"

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
	timeout := config.ExportTimeout
	ui.Say(fmt.Sprintf("Creating  Export timeout is ... %s", timeout))
	if timeout == "" {
		timeout = "2m"
	}

	exportTimeout, _ := time.ParseDuration(timeout)

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
	ui.Say("Waiting for the Export image to SUCCEED...")
	err3 := waitForExportJobToSucceed(config.ImageID, jobId, vpcService, exportTimeout, state)
	if err3 != nil {
		err := fmt.Errorf("[ERROR] Error waiting for the Image export job to succeed: %s", err3)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}
	ui.Say("Image export job succeeded!")
	state.Put("image_id", config.ImageID)
	state.Put("image_export_job_id", jobId)
	ui.Say("Image export job created successfully!") // Image exported job created successfully
	ui.Say(fmt.Sprintf("Image Export Job's ID: %s", *imageExportJob.ID))
	return multistep.ActionContinue
}

func waitForExportJobToSucceed(imageId, exportJobId string, vpcService *vpcv1.VpcV1, timeout time.Duration, state multistep.StateBag) error {
	ui := state.Get("ui").(packer.Ui)
	done := make(chan struct{})
	defer close(done)
	result := make(chan error, 1)

	if timeout < 1*time.Minute {
		timeout = 5 * time.Minute // Default to 45 minutes for image exports
		ui.Say(fmt.Sprintf("Using default %v timeout for image export", timeout))
	} else {
		ui.Say(fmt.Sprintf("Using %v timeout for image export", timeout))
	}

	options := vpcv1.GetImageExportJobOptions{
		ImageID: &imageId,
		ID:      &exportJobId,
	}

	go func() {
		attempts := 0
		for {
			attempts += 1
			if attempts%6 == 0 {
				ui.Say(fmt.Sprintf("Waiting time: %d minutes", attempts/6))
			} else {
				ui.Say(".")
			}

			log.Printf("Checking export job status ... (attempt: %d)", attempts)
			expJob, _, err := vpcService.GetImageExportJob(&options)

			if err != nil {
				ui.Say(fmt.Sprintf("Error fetching image export job: %v", err))
				result <- err
				return
			}

			if expJob.Status != nil {
				log.Printf("Export job status: %s", *expJob.Status)
				if attempts%6 == 0 { // Every 1 minutes
					ui.Say(fmt.Sprintf("Current status: %s", *expJob.Status))
				}
			}

			if expJob.Status != nil && (*expJob.Status == "failed" || *expJob.Status == "deleting") {
				err := fmt.Errorf("export job failed with status: %s", *expJob.Status)
				result <- err
				return
			}

			if expJob.Status != nil && *expJob.Status == "succeeded" {
				result <- nil
				return
			}

			time.Sleep(10 * time.Second)

			select {
			case <-done:
				return
			default:
				// Keep going
			}
		}
	}()

	ui.Say(fmt.Sprintf("Waiting for up to %v for export job to complete", timeout))
	log.Printf("Waiting for up to %d seconds for resource to become ready", int(timeout.Seconds()))

	select {
	case err := <-result:
		return err
	case <-time.After(timeout):
		// Fixed the typo here
		err := fmt.Errorf("timeout while waiting for the resource to become ready after %v", timeout)
		return err
	}
}
func (step *StepImageExport) Cleanup(state multistep.StateBag) {
}
