package ibmcloud

import (
	"github.com/hashicorp/packer/common"
	"github.com/hashicorp/packer/helper/communicator"
	"github.com/hashicorp/packer/template/interpolate"
	"time"
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`
	Comm                communicator.Config `mapstructure:",squash"`

	Username            string   `mapstructure:"username"`
	APIKey              string   `mapstructure:"api_key"`
	ImageName           string   `mapstructure:"image_name"`
	ImageDescription    string   `mapstructure:"image_description"`
	ImageType           string   `mapstructure:"image_type"`
	BaseImageId         string   `mapstructure:"base_image_id"`
	BaseOsCode          string   `mapstructure:"base_os_code"`
	UploadToDatacenters []string `mapstructure:"upload_to_datacenters"`

	InstanceName                   string  `mapstructure:"instance_name"`
	InstanceDomain                 string  `mapstructure:"instance_domain"`
	InstanceFlavor                 string  `mapstructure:"instance_flavor"`
	InstanceLocalDiskFlag          bool    `mapstructure:"instance_local_disk_flag"`
	InstanceCpu                    int     `mapstructure:"instance_cpu"`
	InstanceMemory                 int64   `mapstructure:"instance_memory"`
	InstanceDiskCapacity           int     `mapstructure:"instance_disk_capacity"`
	DatacenterName                 string  `mapstructure:"datacenter_name"`
	PublicVlanId                   int64   `mapstructure:"public_vlan_id"`
	InstanceNetworkSpeed           int     `mapstructure:"instance_network_speed"`
	ProvisioningSshKeyId           int64   `mapstructure:"provisioning_ssh_key_id"`
	InstancePublicSecurityGroupIds []int64 `mapstructure:"public_security_groups"`

	RawStateTimeout string `mapstructure:"instance_state_timeout"`
	StateTimeout    time.Duration

	ctx interpolate.Context
}
