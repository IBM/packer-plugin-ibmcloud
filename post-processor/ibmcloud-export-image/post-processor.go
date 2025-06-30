// Copyright (c) HashiCorp, Inc.
// SPDX-License-Identifier: MPL-2.0

//go:generate packer-sdc mapstructure-to-hcl2 -type Config

package ibmcloudexport

import (
	"context"
	"fmt"
	"packer-plugin-ibmcloud/builder/ibmcloud/vpc"

	// "strings"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	IBMApiKey           string `mapstructure:"api_key"`
	Region              string `mapstructure:"region"`
	Endpoint            string `mapstructure:"vpc_endpoint_url"`
	IAMEndpoint         string `mapstructure:"iam_url"`
	ImageID             string `mapstructure:"image_id"`
	ImageExportJobName  string `mapstructure:"image_export_job_name"`
	ExportTimeout       string `mapstructure:"export_timeout"`

	//The Cloud Object Storage bucket to export the image to. The bucket must exist and an IAM service authorization must grant Image Service for VPC of VPC Infrastructure Services writer access to the bucket.
	StorageBucketName string `mapstructure:"storage_bucket_name"`
	StorageBucketCRN  string `mapstructure:"storage_bucket_crn"`

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
			p.config.Endpoint = "https://" + p.config.Region + ".iaas.cloud.ibm.com/v1/"
		}
	} else {
		if p.config.IBMApiKey != "" {
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("api_key must not be provided when image_id is not given.."))
		}
		if p.config.Region != "" {
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("region must not be provided when image_id is not given.."))
		}
		if p.config.Endpoint != "" {
			errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("vpc_endpoint_url must not be provided when image_id is not given.."))
		}
	}
	if p.config.StorageBucketName == "" && p.config.StorageBucketCRN == "" {
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("either storage_bucket_name or storage_bucket_crn must be provided.."))
	}
	if p.config.StorageBucketName != "" && p.config.StorageBucketCRN != "" {
		errs = packersdk.MultiErrorAppend(errs, fmt.Errorf("storage_bucket_name and storage_bucket_crn cann't be provided together.."))
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
	switch source.BuilderId() {
	case vpc.BuilderId, "ibmcloud.post-processor.vpc-export":
		break
	default:
		err := fmt.Errorf(
			"Unknown artifact type: %s\nCan only export from IBM Cloud Engine Builder and Artifice post-processor artifacts. ",
			source.BuilderId())
		return nil, false, false, err
	}
	ibmApiKey := source.State("ibmApiKey").(string)
	region := source.State("region").(string)
	vpc_endpoint_url := source.State("vpc_endpoint_url").(string)
	iam_url := source.State("iam_url").(string)
	imageId := source.State("image_id").(string)
	imageName := source.State("image_name").(string)

	if p.config.ImageID == "" {
		// take info from source
		p.config.IBMApiKey = ibmApiKey
		p.config.Region = region
		p.config.Endpoint = vpc_endpoint_url
		p.config.IAMEndpoint = iam_url
		p.config.ImageID = imageId
	}

	exporterConfig := vpc.Config{
		IBMApiKey:          p.config.IBMApiKey,
		Region:             p.config.Region,
		Endpoint:           p.config.Endpoint,
		IAMEndpoint:        p.config.IAMEndpoint,
		ImageID:            p.config.ImageID,
		ImageExportJobName: p.config.ImageExportJobName,
		ExportTimeout:      p.config.ExportTimeout,
		StorageBucketName:  p.config.StorageBucketName,
		StorageBucketCRN:   p.config.StorageBucketCRN,
		Format:             p.config.Format,
	}
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
	if err, ok := state.GetOk("error"); ok {
		return nil, false, false, err.(error)
	}

	// Create an artifact and return it
	result := &Artifact{
		imageExportJobId: state.Get("image_export_job_id").(string),
		imageId:          imageId,
		imageName:        imageName,
		// Add the builder generated data to the artifact StateData so that post-processors can access them.
		StateData: map[string]interface{}{
			"ibmApiKey":        ibmApiKey,
			"region":           region,
			"vpc_endpoint_url": vpc_endpoint_url,
			"iam_url":          iam_url,
			"image_id":         imageId,
			"image_name":       imageName,
		},
	}
	return result, false, false, nil
}
