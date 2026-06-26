//go:generate packer-sdc mapstructure-to-hcl2 -type Config
package vpc

import (
	"errors"
	"fmt"
	"os"
	"slices"
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

	IBMApiKey                 string `mapstructure:"api_key"`
	Region                    string `mapstructure:"region"`
	Endpoint                  string `mapstructure:"vpc_endpoint_url"`
	RCEndpoint                string `mapstructure:"rc_endpoint_url"`
	GhostEndpoint             string `mapstructure:"ghost_endpoint_url"`
	EncryptionKeyCRN          string `mapstructure:"encryption_key_crn"`
	IAMEndpoint               string `mapstructure:"iam_url"`
	Zone                      string `mapstructure-to-hcl2:",skip"`
	VPCID                     string `mapstructure-to-hcl2:",skip"`
	SubnetID                  string `mapstructure:"subnet_id"`
	SshKeyType                string `mapstructure:"ssh_key_type"`
	CatalogOfferingCRN        string `mapstructure:"catalog_offering_crn"`
	CatalogOfferingVersionCRN string `mapstructure:"catalog_offering_version_crn"`
	ResourceGroupID           string `mapstructure:"resource_group_id"`
	ResourceGroupName         string `mapstructure:"resource_group_name"`
	SecurityGroupID           string `mapstructure:"security_group_id"`
	VSIBaseImageID            string `mapstructure:"vsi_base_image_id"`
	VSIBaseImageName          string `mapstructure:"vsi_base_image_name"`
	VSIBootCapacity           int    `mapstructure:"vsi_boot_vol_capacity"`
	VSIBootProfile            string `mapstructure:"vsi_boot_vol_profile"`
	VSIBootIops               int    `mapstructure:"vsi_boot_vol_iops"`
	VSIBootBandwidth          int    `mapstructure:"vsi_boot_vol_bandwidth"`
	VSIBootVolumeID           string `mapstructure:"vsi_boot_volume_id"`
	VSIBootSnapshotID         string `mapstructure:"vsi_boot_snapshot_id"`
	VSIDataCapacity           int    `mapstructure:"vsi_data_vol_capacity"`
	VSIDataProfile            string `mapstructure:"vsi_data_vol_profile"`
	VSIDataIops               int    `mapstructure:"vsi_data_vol_iops"`
	VSIDataBandwidth          int    `mapstructure:"vsi_data_vol_bandwidth"`
	VSIProfile                string `mapstructure:"vsi_profile"`
	VSIInterface              string `mapstructure:"vsi_interface"`
	VSIUserDataFile           string `mapstructure:"vsi_user_data_file"`
	VSIUserDataString         string `mapstructure:"vsi_user_data"`

	ImageName string   `mapstructure:"image_name"`
	ImageTags []string `mapstructure:"tags"`

	// Security Group Rule Configuration
	SkipCreateDefaultSecurityGroupRule bool     `mapstructure:"skip_create_default_security_group_rule"`
	SecurityGroupRuleRemoteCIDR        []string `mapstructure:"security_group_rule_remote_cidr"`
	SecurityGroupRuleRemoteAddress     []string `mapstructure:"security_group_rule_remote_address"`
	SecurityGroupRuleRemoteID          []string `mapstructure:"security_group_rule_remote_id"`

	VSIName           string `mapstructure-to-hcl2:",skip"`
	VpcSshKeyName     string `mapstructure-to-hcl2:",skip"`
	SecurityGroupName string `mapstructure-to-hcl2:",skip"`
	FloatingIPName    string `mapstructure-to-hcl2:",skip"`

	RawStateTimeout string              `mapstructure:"timeout"`
	StateTimeout    time.Duration       `mapstructure-to-hcl2:",skip"`
	ctx             interpolate.Context `mapstructure-to-hcl2:",skip"`

	ImageID            string `mapstructure:"image_id"`
	ImageExportJobName string `mapstructure:"image_export_job_name"`
	ExportTimeout      string `mapstructure:"export_timeout"`
	//The Cloud Object Storage bucket to export the image to. The bucket must exist and an IAM service authorization must grant Image Service for VPC of VPC Infrastructure Services writer access to the bucket.
	StorageBucketName string `mapstructure:"storage_bucket_name"`
	StorageBucketCRN  string `mapstructure:"storage_bucket_crn"`
	//The format to use for the exported image. If the image is encrypted, only qcow2 is supported.
	Format     string `mapstructure:"format"`
	SkipReboot bool   `mapstructure:"skip_reboot"`
	VPCLog     string `mapstructure:"logging"`
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
	if c.RCEndpoint == "" {
		c.RCEndpoint = "https://resource-controller.cloud.ibm.com"
	}

	if c.SubnetID == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("a subnet_id must be specified"))
	}

	if c.VSIBootCapacity != 0 && (c.VSIBootCapacity < 10 || c.VSIBootCapacity > 32000) {
		errs = packer.MultiErrorAppend(errs, errors.New("boot capacity out of bound: provide a valid capacity between 10 and 32000"))
	}
	allowedBootProfiles := []string{"general-purpose", "5iops-tier", "10iops-tier", "sdp", "custom"}
	if c.VSIBootProfile != "" && !slices.Contains(allowedBootProfiles, c.VSIBootProfile) {
		errs = packer.MultiErrorAppend(errs, errors.New("vsi_boot_vol_profile must be one of: general-purpose, 5iops-tier, 10iops-tier, sdp, custom"))
	}
	// iops/bandwidth are only honored by the custom and sdp profiles; the tiered
	// profiles derive them from capacity. This validation is the single source of
	// truth for that rule (the bootVolumePrototype helpers do not re-enforce it).
	customOrSdp := c.VSIBootProfile == "custom" || c.VSIBootProfile == "sdp"
	if (c.VSIBootIops != 0 || c.VSIBootBandwidth != 0) && !customOrSdp {
		errs = packer.MultiErrorAppend(errs, errors.New("vsi_boot_vol_iops/vsi_boot_vol_bandwidth require vsi_boot_vol_profile to be 'custom' or 'sdp'"))
	}
	bootVolumeTuned := c.VSIBootProfile != "" || c.VSIBootIops != 0 || c.VSIBootBandwidth != 0
	// The by-image and catalog-offering paths only attach a boot volume (and thus
	// only honor profile/iops/bandwidth) when a capacity is set; without it those
	// settings would be silently dropped, so require capacity there. The snapshot
	// path is exempt: snapshotBootVolumePrototype lets the restored volume inherit
	// the snapshot's size when capacity is unset.
	if c.VSIBootSnapshotID == "" && bootVolumeTuned && c.VSIBootCapacity == 0 {
		errs = packer.MultiErrorAppend(errs, errors.New("vsi_boot_vol_profile/vsi_boot_vol_iops/vsi_boot_vol_bandwidth require vsi_boot_vol_capacity to be set"))
	}
	// Attaching an existing volume by ID uses that volume's own profile, iops,
	// bandwidth, and capacity; specifying any of them here would be silently
	// ignored, so reject the combination rather than mislead the user.
	if c.VSIBootVolumeID != "" && (bootVolumeTuned || c.VSIBootCapacity != 0) {
		errs = packer.MultiErrorAppend(errs, errors.New("vsi_boot_vol_profile/vsi_boot_vol_iops/vsi_boot_vol_bandwidth/vsi_boot_vol_capacity cannot be combined with vsi_boot_volume_id; the existing volume's properties are used"))
	}
	if c.VSIBootIops < 0 || c.VSIBootBandwidth < 0 {
		errs = packer.MultiErrorAppend(errs, errors.New("vsi_boot_vol_iops and vsi_boot_vol_bandwidth must not be negative"))
	}

	// Data-volume validation mirrors the boot-volume rules above. The data volume
	// is an ephemeral scratch disk attached to the builder VSI and deleted with it
	// (DeleteVolumeOnInstanceDelete); it never enters the captured image, which is
	// taken from the boot volume only. It lets a build keep large transient writes
	// (build caches, downloads) off the boot volume so they are not exported at
	// capture time.
	if c.VSIDataCapacity != 0 && (c.VSIDataCapacity < 10 || c.VSIDataCapacity > 32000) {
		errs = packer.MultiErrorAppend(errs, errors.New("data capacity out of bound: provide a valid capacity between 10 and 32000"))
	}
	allowedDataProfiles := []string{"general-purpose", "5iops-tier", "10iops-tier", "sdp", "custom"}
	if c.VSIDataProfile != "" && !slices.Contains(allowedDataProfiles, c.VSIDataProfile) {
		errs = packer.MultiErrorAppend(errs, errors.New("vsi_data_vol_profile must be one of: general-purpose, 5iops-tier, 10iops-tier, sdp, custom"))
	}
	// iops/bandwidth are honored only by the custom and sdp profiles (the tiered
	// profiles derive them from capacity), same rule as the boot volume above.
	dataCustomOrSdp := c.VSIDataProfile == "custom" || c.VSIDataProfile == "sdp"
	if (c.VSIDataIops != 0 || c.VSIDataBandwidth != 0) && !dataCustomOrSdp {
		errs = packer.MultiErrorAppend(errs, errors.New("vsi_data_vol_iops/vsi_data_vol_bandwidth require vsi_data_vol_profile to be 'custom' or 'sdp'"))
	}
	// The data volume is attached only when a capacity is set; without it the
	// profile/iops/bandwidth would be silently dropped, so require capacity.
	dataVolumeTuned := c.VSIDataProfile != "" || c.VSIDataIops != 0 || c.VSIDataBandwidth != 0
	if dataVolumeTuned && c.VSIDataCapacity == 0 {
		errs = packer.MultiErrorAppend(errs, errors.New("vsi_data_vol_profile/vsi_data_vol_iops/vsi_data_vol_bandwidth require vsi_data_vol_capacity to be set"))
	}
	if c.VSIDataIops < 0 || c.VSIDataBandwidth < 0 {
		errs = packer.MultiErrorAppend(errs, errors.New("vsi_data_vol_iops and vsi_data_vol_bandwidth must not be negative"))
	}

	var oneOfInput int // validation for mutually exclusive fields.

	if c.VSIBaseImageID != "" {
		oneOfInput = oneOfInput + 1
	}
	if c.VSIBaseImageName != "" {
		oneOfInput = oneOfInput + 1
	}
	if c.CatalogOfferingCRN != "" {
		oneOfInput = oneOfInput + 1
	}
	if c.CatalogOfferingVersionCRN != "" {
		oneOfInput = oneOfInput + 1
	}
	if c.VSIBootVolumeID != "" {
		oneOfInput = oneOfInput + 1
	}
	if c.VSIBootSnapshotID != "" {
		oneOfInput = oneOfInput + 1
	}

	if oneOfInput != 1 {
		errs = packer.MultiErrorAppend(errs, errors.New("only one of (vsi_base_image_id or vsi_base_image_name) or (catalog_offering_crn or catalog_offering_version_crn) or vsi_boot_volume_id or vsi_boot_snapshot_id is required"))
	}

	if c.VSIProfile == "" {
		errs = packer.MultiErrorAppend(errs, errors.New("a vsi_profile must be specified"))
	}

	if c.VSIInterface == "" {
		c.VSIInterface = "public"
	}

	// Check for mutual exclusion of User data input via file or as a string.
	if c.VSIUserDataFile != "" && c.VSIUserDataString != "" {
		errs = packer.MultiErrorAppend(
			errs, errors.New("mutual exclusion: User data input, either a file as in vsi_user_data_file could be used or a string in vsi_user_data_string, together are not supported"))
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
