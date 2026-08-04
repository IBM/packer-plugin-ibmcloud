package vpc

import (
	"context"
	"fmt"
	"math/rand"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// subnetZone pairs a subnet id with the zone it lives in. stepCreateInstance
// walks the slice (already shuffled here) creating the builder VSI in each
// subnet's zone until one starts (see the capacity fallback there).
type subnetZone struct {
	ID   string
	Zone string
}

type stepGetSubnetInfo struct{}

func (s *stepGetSubnetInfo) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)
	svc := vpcService(state)

	// config.SubnetIDs is the normalized list; a lone subnet_id became a
	// one-element list in Config.Prepare.
	subnets := make([]subnetZone, 0, len(config.SubnetIDs))
	var vpcID string
	for _, subnetID := range config.SubnetIDs {
		ui.Say(fmt.Sprintf("Retrieving Subnet %s information...", subnetID))

		options := &vpcv1.GetSubnetOptions{}
		options.SetID(subnetID)
		subnetData, _, err := svc.GetSubnet(options)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error fetching subnet %s: %s", subnetID, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		if subnetData.VPC == nil || subnetData.VPC.ID == nil || subnetData.Zone == nil || subnetData.Zone.Name == nil {
			err := fmt.Errorf("[ERROR] Subnet %s is missing VPC or zone information", subnetID)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		// All subnets must share one VPC: the VPC identity and the security group
		// are fixed for the build, so a subnet in a different VPC could not serve
		// as a fallback.
		subnetVPC := *subnetData.VPC.ID
		if vpcID == "" {
			vpcID = subnetVPC
		} else if subnetVPC != vpcID {
			err := fmt.Errorf("[ERROR] All subnets must belong to the same VPC; subnet %s is in VPC %s but an earlier subnet is in VPC %s", subnetID, subnetVPC, vpcID)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		zone := *subnetData.Zone.Name
		subnets = append(subnets, subnetZone{ID: subnetID, Zone: zone})
		ui.Say(fmt.Sprintf("Subnet %s is in zone %s", subnetID, zone))
	}

	if config.SecurityGroupID != "" { // User provided security group
		secGrpVPC := state.Get("user_sec_grp_vpc")
		ui.Say("Verifying the security group and subnets belong to the same VPC..")
		if vpcID != secGrpVPC {
			err := fmt.Errorf("The security group and subnets provided are not connected to the same VPC id: %s", vpcID)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}

	// Try the subnets in a random order so repeated builds spread their baseline
	// load across zones instead of always starting in the same one (which itself
	// drives capacity pressure). Capacity fallback still walks the whole list.
	rand.Shuffle(len(subnets), func(i, j int) { subnets[i], subnets[j] = subnets[j], subnets[i] })

	state.Put("vpc_id", vpcID)
	state.Put("bake_subnets", subnets)

	ui.Say("Subnet Information successfully retrieved ...")
	ui.Say(fmt.Sprintf("VPC ID: %s", vpcID))

	return multistep.ActionContinue
}

func (s *stepGetSubnetInfo) Cleanup(state multistep.StateBag) {

}
