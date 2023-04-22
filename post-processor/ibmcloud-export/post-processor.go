// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package ibmcloudexport

import (
	"context"
	"fmt"
	"packer-plugin-ibmcloud/builder/ibmcloud/vpc"
	"strings"
	"time"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

type Config struct {

	//A temporary OAuth 2.0 access token
	// AccessToken string `mapstructure:"access_token" required:"false"`

	common.PackerConfig `mapstructure:",squash"`
	IBMApiKey                 string `mapstructure:"api_key"`
	Region                    string `mapstructure:"region"`
	Endpoint                  string `mapstructure:"vpc_endpoint_url"`
	IAMEndpoint               string `mapstructure:"iam_url"`
	ImageID            string `mapstructure:"image_id"`
	ImageExportJobName string `mapstructure:"image_export_job_name"`

	//The Cloud Object Storage bucket to export the image to. The bucket must exist and an IAM service authorization must grant Image Service for VPC of VPC Infrastructure Services writer access to the bucket.
	StorageBucketName string `mapstructure:"storage_bucket_name" required:"true"`

	//The format to use for the exported image. If the image is encrypted, only qcow2 is supported.
	Format string `mapstructure:"format"`

	ctx interpolate.Context
}

type PostProcessor struct {
	config Config
	runner multistep.Runner
}

func (p *PostProcessor) ConfigSpec() hcldec.ObjectSpec { return p.config.FlatMapstructure().HCL2Spec() }

func (p *PostProcessor) Configure(raws ...interface{}) error {
	err := config.Decode(&p.config, &config.DecodeOpts{
		PluginType:         "ibmcloud.post-processor.vpc-export",
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter:  &interpolate.RenderFilter{},
	}, raws...)
	if err != nil {
		return err
	}
	errs := new(packersdk.MultiError)

	if p.config.ImageID != "" {
		if p.config.IBMApiKey == "" {
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("api_key must be provided when image_id is given.."))
		}
		if p.config.Region == "" {
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("region must be provided when image_id is given.."))
		}
		if p.config.Endpoint == "" {
			p.config.Endpoint = "https://" + c.Region + ".iaas.cloud.ibm.com/v1/"
		}
		// if p.config.IAMEndpoint == "" {
		// 	errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("iam_url must be provided when image_id is given.."))
		// }
	}
	if p.config.StorageBucketName == "" {
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("Storage Bucket Name must be provided.."))
	}

	currentTime := time.Now()
	// Set defaults.
	if p.config.ImageExportJobName == "" {
		p.config.ImageExportJobName = fmt.Sprintf("ibm-packer-%d-exported-image", currentTime.Unix())
	}

	if p.config.Format == "" {
		p.config.ImageExportJobName = "qcow2"
	}

	if len(errs.Errors) > 0 {
		return errs
	}
	return nil
}

func (p *PostProcessor) PostProcess(ctx context.Context, ui packersdk.Ui, source packersdk.Artifact) (packersdk.Artifact, bool, bool, error) {
	ui.Say(fmt.Sprintf("post-processor begins!!!.."))

	switch source.BuilderId() {
	case vpc.BuilderId, "ibmcloud.post-processor.vpc-export":
		break
	default:
		err := fmt.Errorf(
			"Unknown artifact type: %s\nCan only export from IBM Cloud Engine Builder and Artifice post-processor artifacts. ",
			source.BuilderId())
		return nil, false, false, err
	}

	if p.config.ImageID == "" {
		if source.Id() != "" {
			p.config.ImageID = source.Id()
		} else {
			err := fmt.Errorf(
				"Unknown Image Id: \n Please provide value in image_id parameter ")
			return nil, false, false, err
		}
	}

	ui.Say("source.String():API KEY VALUE")
	ui.Say(strings.TrimSpace(strings.Split(strings.Split(source.String(), "||")[2], ":")[1]))
	iBMApiKey := strings.TrimSpace(strings.Split(strings.Split(source.String(), "||")[2], ":")[1])
	if p.config.IBMApiKey == "" {
		if iBMApiKey != "" {
			p.config.IBMApiKey = iBMApiKey
		} else {
			err := fmt.Errorf(
				"Unknown IBM API KEY : \n Please provide value in api_key parameter ")
			return nil, false, false, err
		}
	}

	exporterConfig := vpc.Config{
		IBMApiKey:          p.config.IBMApiKey,
		ImageID:            p.config.ImageID,
		ImageExportJobName: p.config.ImageExportJobName,
		StorageBucketName:  p.config.StorageBucketName,
		Format:             p.config.Format,
	}
	ui.Say("source.BuilderId()")
	ui.Say(fmt.Sprintf("%s", source.BuilderId()))

	ui.Say("source.Id()")
	ui.Say(source.Id())

	ui.Say("source.String()")
	ui.Say(strings.TrimSpace(strings.Split(strings.Split(source.String(), "||")[1], ":")[1]))

	ui.Say(fmt.Sprintf("Exporting image %v to destination: %v", source.Id(), p.config.StorageBucketName))

	client := vpc.IBMCloudClient{}.New(p.config.IBMApiKey)

	// Set up the state which is used to share state between the steps
	state := new(multistep.BasicStateBag)
	state.Put("config", exporterConfig)
	state.Put("client", client)
	state.Put("ui", ui)

	// Build the steps
	steps := []multistep.Step{}
	steps = []multistep.Step{
		new(vpc.StepGreeting),
		new(vpc.StepCreateVPCServiceInstance),
		new(vpc.StepImageExport),
	}
	p.runner = &multistep.BasicRunner{Steps: steps}
	p.runner.Run(ctx, state)

	// If there was an error, return that
	if _, ok := state.GetOk("error"); ok {
		ui.Say(fmt.Sprintf("Error occured"))
	}

	generatedData := source.State("generated_data")
	if generatedData == nil {
		// Make sure it's not a nil map so we can assign to it later.
		generatedData = make(map[string]interface{})
	}
	p.config.ctx.Data = generatedData

	// Create an artifact and return it
	result := &Artifact{
		imageeExportJobId: state.Get("image_export_job_id").(string),
		imageId:           state.Get("image_id").(string),
		// Add the builder generated data to the artifact StateData so that post-processors can access them.
		StateData: map[string]interface{}{"generated_data": state.Get("generated_data")},
	}

	ui.Say(fmt.Sprintf("I AM BACK FROM CREATING EXPORT JOBS"))

	return result, false, false, nil

	// return source, true, true, nil
}
