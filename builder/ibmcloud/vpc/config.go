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

	IBMApiKey       string `mapstructure:"api_key"`
	Region          string `mapstructure:"region"`
	EndPoint        string `mapstructure-to-hcl2:",skip"`
	Zone            string `mapstructure-to-hcl2:",skip"`
	Version         string `mapstructure-to-hcl2:",skip"`
	Generation      string `mapstructure-to-hcl2:",skip"`
	VPCID           string `mapstructure-to-hcl2:",skip"`
	SubnetID        string `mapstructure:"subnet_id"`
	ResourceGroupID string `mapstructure:"resource_group_id"`
	SecurityGroupID string `mapstructure:"security_group_id"`
	VSIBaseImageID  string `mapstructure:"vsi_base_image_id"`
	VSIProfile      string `mapstructure:"vsi_profile"`
	VSIInterface    string `mapstructure:"vsi_interface"`
	VSIUserDataFile string `mapstructure:"vsi_user_data_file"`

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

	// Configure IBM Cloud EndPoint and other IBM Cloud API constants
	c.EndPoint = "https://" + c.Region + ".iaas.cloud.ibm.com/v1/"
	c.Version = fmt.Sprintf("version=%d-%02d-%02d", currentTime.Year(), currentTime.Month(), currentTime.Day())
	c.Generation = "generation=2"
	// log.Println("Version : ", c.Version)

	if c.SubnetID == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("a subnet_id must be specified"))
	}

	if c.VSIBaseImageID == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("a vsi_base_image_id must be specified"))
	}

	if c.VSIProfile == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("a vsi_profile must be specified"))
	}

	if c.VSIInterface == "" {
		c.VSIInterface = "public"
	}

	if c.VSIUserDataFile != "" && c.Comm.Type == "winrm" {
		if _, err := os.Stat(c.VSIUserDataFile); os.IsNotExist(err) {
			errs = packer.MultiErrorAppend(
				errs, fmt.Errorf("failed to read user-data-file: %s", err))
		}
	}

	if c.ImageName == "" {
		c.ImageName = fmt.Sprintf("packer-vpc-%d", currentTime.Unix())
	} else {
		c.ImageName = fmt.Sprintf("%s-%d%d%d", c.ImageName, currentTime.Hour(), currentTime.Minute(), currentTime.Second())
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
	c.VSIName = fmt.Sprintf("%s-vsi-%d%d%d", UniqueID, currentTime.Hour(), currentTime.Minute(), currentTime.Second())
	c.VpcSshKeyName = fmt.Sprintf("%s-ssh-key-%d%d%d", UniqueID, currentTime.Hour(), currentTime.Minute(), currentTime.Second())
	c.SecurityGroupName = fmt.Sprintf("%s-security-group-%d%d%d", UniqueID, currentTime.Hour(), currentTime.Minute(), currentTime.Second())
	c.FloatingIPName = fmt.Sprintf("%s-floating-ip-%d%d%d", UniqueID, currentTime.Hour(), currentTime.Minute(), currentTime.Second())

	if errs != nil && len(errs.Errors) > 0 {
		return nil, errs
	}

	return nil, nil
}
