package vpc

import (
	"context"
	"fmt"
	"strings"

	searchv2 "github.com/IBM/platform-services-go-sdk/globalsearchv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepGetSubnetInfo struct{}

func (s *stepGetSubnetInfo) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	ui.Say(fmt.Sprintf("Retrieving Subnet %s information...", config.SubnetID))

	options := &vpcv1.GetSubnetOptions{}
	options.SetID(config.SubnetID)
	subnetData, _, err := vpcService.GetSubnet(options)

	if err != nil {
		err := fmt.Errorf("[ERROR] Error fetching subnet %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}

	vpcId := *subnetData.VPC.ID
	zone := *subnetData.Zone.Name

	state.Put("vpc_id", vpcId)
	state.Put("zone", zone)

	ui.Say("Subnet Information successfully retrieved ...")
	ui.Say(fmt.Sprintf("VPC ID: %s", vpcId))
	ui.Say(fmt.Sprintf("Zone: %s", zone))

	if config.CatalogOfferingCRN != "" || config.CatalogOfferingVersionCRN != "" || config.EncryptionKeyCRN != "" {
		// validate crn

		searchURL := "https://api.global-search-tagging.cloud.ibm.com"
		globalSearchV2Options := &searchv2.GlobalSearchV2Options{
			URL:           searchURL,
			Authenticator: vpcService.Service.Options.Authenticator,
		}
		globalSearchAPIV2, err := searchv2.NewGlobalSearchV2(globalSearchV2Options)

		if err != nil {
			fmt.Println("Gen2 Service creation failed.", err)
		}
		// validate catalog offering crn
		if config.CatalogOfferingCRN != "" {
			crnToCheck := fmt.Sprintf("%s%s", strings.Split(config.CatalogOfferingCRN, ":offering")[0], "::")
			query := fmt.Sprintf("crn:\"%s\"", crnToCheck)
			isHidden := "any"
			searchOptions := &searchv2.SearchOptions{
				Query:    &query,
				IsHidden: &isHidden,
			}
			res, _, _ := globalSearchAPIV2.Search(searchOptions)
			if len(res.Items) != 0 {
				ui.Say(fmt.Sprintf("%s Catalog information successfully retrieved ...", res.Items[0].GetProperty("name")))
			} else {
				state.Put("Catalog information could not be retrieved", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
		}
		// validate catalog version crn
		if config.CatalogOfferingVersionCRN != "" {
			crnToCheck := fmt.Sprintf("%s%s", strings.Split(config.CatalogOfferingVersionCRN, ":version")[0], "::")
			query := fmt.Sprintf("crn:\"%s\"", crnToCheck)
			isHidden := "any"
			searchOptions := &searchv2.SearchOptions{
				Query:    &query,
				IsHidden: &isHidden,
			}
			res, _, _ := globalSearchAPIV2.Search(searchOptions)
			if len(res.Items) != 0 {
				ui.Say(fmt.Sprintf("%s Catalog information successfully retrieved ...", res.Items[0].GetProperty("name")))
			} else {
				state.Put("Catalog information could not be retrieved", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
		}
		// validate encryption key crn
		if config.EncryptionKeyCRN != "" {
			crnToCheck := fmt.Sprintf("%s%s", strings.Split(config.EncryptionKeyCRN, ":key")[0], "::")
			query := fmt.Sprintf("crn:\"%s\"", crnToCheck)
			isHidden := "any"
			searchOptions := &searchv2.SearchOptions{
				Query:    &query,
				IsHidden: &isHidden,
			}
			res, _, _ := globalSearchAPIV2.Search(searchOptions)
			if len(res.Items) != 0 {
				ui.Say(fmt.Sprintf("%s Encryption information successfully retrieved ...", res.Items[0].GetProperty("name")))
			} else {
				state.Put("Encryption information could not be retrieved", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
		}
	}

	return multistep.ActionContinue
}

func (s *stepGetSubnetInfo) Cleanup(state multistep.StateBag) {

}
