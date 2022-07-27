//go:generate packer-sdc mapstructure-to-hcl2 -type Config
package vpc

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/common"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/template/config"
	"github.com/hashicorp/packer-plugin-sdk/template/interpolate"
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	Comm                communicator.Config `mapstructure:",squash"`

	IBMApiKey        string `mapstructure:"api_key"`
	Region           string `mapstructure:"region"`
	Endpoint         string `mapstructure:"vpc_endpoint_url"`
	IAMEndpoint      string `mapstructure:"iam_url"`
	Zone             string `mapstructure-to-hcl2:",skip"`
	VPCID            string `mapstructure-to-hcl2:",skip"`
	SubnetID         string `mapstructure:"subnet_id"`
	ResourceGroupID  string `mapstructure:"resource_group_id"`
	SecurityGroupID  string `mapstructure:"security_group_id"`
	VSIBaseImageID   string `mapstructure:"vsi_base_image_id"`
	VSIBaseImageName string `mapstructure:"vsi_base_image_name"`
	VSIProfile       string `mapstructure:"vsi_profile"`
	VSIInterface     string `mapstructure:"vsi_interface"`
	VSIUserDataFile  string `mapstructure:"vsi_user_data_file"`

	ImageName string `mapstructure:"image_name"`

	VSIName           string `mapstructure-to-hcl2:",skip"`
	VpcSshKeyName     string `mapstructure-to-hcl2:",skip"`
	SecurityGroupName string `mapstructure-to-hcl2:",skip"`
	FloatingIPName    string `mapstructure-to-hcl2:",skip"`

	RawStateTimeout string              `mapstructure:"timeout"`
	StateTimeout    time.Duration       `mapstructure-to-hcl2:",skip"`
	ctx             interpolate.Context `mapstructure-to-hcl2:",skip"`
}

// Prepare processes the build configuration parameters.
func (c *Config) Prepare(raws ...interface{}) ([]string, error) {
	err := config.Decode(c, &config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &c.ctx,
		InterpolateFilter:  &interpolate.RenderFilter{},
	}, raws...)

	if err != nil {
		return nil, err
	}

	currentTime := time.Now()

	// Check for required configurations that will display errors if not specified
	var errs *packer.MultiError
	errs = packer.MultiErrorAppend(errs, c.Comm.Prepare(&c.ctx)...)

	if c.IBMApiKey == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("an ibm_api_key must be specified"))
	}

	if c.Region == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("a region must be specified"))
	}

	// Configure IBM Cloud Endpoint and other IBM Cloud API constants
	if c.Endpoint == "" {
		c.Endpoint = "https://" + c.Region + ".iaas.cloud.ibm.com/v1/"
	}

	if c.SubnetID == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("a subnet_id must be specified"))
	}

	if c.VSIBaseImageID == "" && c.VSIBaseImageName == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("a vsi_base_image_id or vsi_base_image_name must be specified"))
	}

	if c.VSIProfile == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("a vsi_profile must be specified"))
	}

	if c.VSIInterface == "" {
		c.VSIInterface = "public"
	}

	if c.VSIUserDataFile != "" {
		if _, err := os.Stat(c.VSIUserDataFile); os.IsNotExist(err) {
			errs = packer.MultiErrorAppend(
				errs, fmt.Errorf("failed to read user-data-file: %s", err))
		}
	}

	if c.ImageName == "" {
		c.ImageName = fmt.Sprintf("packer-vpc-%d", currentTime.Unix())
	}

	if c.Comm.Type == "winrm" {
		if c.Comm.WinRMUser == "" {
			c.Comm.WinRMUser = "Administrator"
		}
	} else if c.Comm.Type == "ssh" {
		if c.Comm.SSHUsername == "" {
			c.Comm.SSHUsername = "root"
		}
	}

	if c.RawStateTimeout == "" {
		c.RawStateTimeout = "2m"
	}

	StateTimeout, err := time.ParseDuration(c.RawStateTimeout)
	if err != nil {
		errs = packer.MultiErrorAppend(
			errs, fmt.Errorf("failed parsing vsi timeout: %s", err))
	}
	c.StateTimeout = StateTimeout

	// Naming temporary infrastructure created during packer execution
	UniqueID := "packer-vpc"
	timestamp := time.Now().UnixNano()
	c.VSIName = fmt.Sprintf("%s-vsi-%d", UniqueID, timestamp)
	c.VpcSshKeyName = fmt.Sprintf("%s-ssh-key-%d", UniqueID, timestamp)
	c.SecurityGroupName = fmt.Sprintf("%s-security-group-%d", UniqueID, timestamp)
	c.FloatingIPName = fmt.Sprintf("%s-floating-ip-%d", UniqueID, timestamp)

	if errs != nil && len(errs.Errors) > 0 {
		return nil, errs
	}

	return nil, nil
}
