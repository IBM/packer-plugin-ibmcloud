package vpc

import (
	"encoding/json"
	"os"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

type trackedResources struct {
	VPCSSHKeyID         string `json:"vpc_ssh_key_id,omitempty"`
	VPCSSHKeyName       string `json:"vpc_ssh_key_name,omitempty"`
	InstanceID          string `json:"instance_id,omitempty"`
	InstanceName        string `json:"instance_name,omitempty"`
	FloatingIPID        string `json:"floating_ip_id,omitempty"`
	FloatingIP          string `json:"floating_ip,omitempty"`
	SecurityGroupID     string `json:"security_group_id,omitempty"`
	SecurityGroupName   string `json:"security_group_name,omitempty"`
	SecurityGroupRuleID string `json:"security_group_rule_id,omitempty"`
	ImageID             string `json:"image_id,omitempty"`
	ResourceGroupID     string `json:"resource_group_id,omitempty"`
	SubnetID            string `json:"subnet_id,omitempty"`
	Region              string `json:"region,omitempty"`
}

func writeTrackedResources(state multistep.StateBag) error {
	config := state.Get("config").(Config)
	if config.ResourceTrackFile == "" {
		return nil
	}

	resources := trackedResources{
		SubnetID: config.SubnetID,
		Region:   config.Region,
	}
	if config.ResourceGroupID != "" {
		resources.ResourceGroupID = config.ResourceGroupID
	} else if derivedResourceGroupID, ok := state.GetOk("derived_resource_group_id"); ok {
		if resourceGroupID, ok := derivedResourceGroupID.(string); ok {
			resources.ResourceGroupID = resourceGroupID
		}
	}
	if value, ok := state.GetOk("vpc_ssh_key_id"); ok {
		resources.VPCSSHKeyID = value.(string)
	}
	if value, ok := state.GetOk("vpc_ssh_key_name"); ok {
		resources.VPCSSHKeyName = value.(string)
	}
	if value, ok := state.GetOk("instance_data"); ok {
		instanceData := value.(*vpcv1.Instance)
		resources.InstanceID = *instanceData.ID
		resources.InstanceName = *instanceData.Name
	}
	if value, ok := state.GetOk("floating_ip_id"); ok {
		resources.FloatingIPID = value.(string)
	}
	if value, ok := state.GetOk("floating_ip"); ok {
		resources.FloatingIP = value.(string)
	}
	if value, ok := state.GetOk("security_group_id"); ok {
		resources.SecurityGroupID = value.(string)
	}
	if value, ok := state.GetOk("security_group_name"); ok {
		resources.SecurityGroupName = value.(string)
	}
	if value, ok := state.GetOk("security_group_rule_id"); ok {
		resources.SecurityGroupRuleID = value.(string)
	}
	if value, ok := state.GetOk("image_id"); ok {
		resources.ImageID = value.(string)
	}

	data, err := json.MarshalIndent(resources, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(config.ResourceTrackFile, data, 0644)
}
