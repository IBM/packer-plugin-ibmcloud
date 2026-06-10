package vpc

import (
	"context"
	"fmt"
	"strings"

	"github.com/IBM/go-sdk-core/v5/core"
	searchv2 "github.com/IBM/platform-services-go-sdk/globalsearchv2"

	"github.com/IBM/platform-services-go-sdk/resourcemanagerv2"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type stepVerifyInput struct{}

func (s *stepVerifyInput) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	client := state.Get("client").(*IBMCloudClient)
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)
	// vpc service
	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}
	// region check
	getRegionOptions := &vpcv1.GetRegionOptions{
		Name: &config.Region,
	}
	_, _, err := vpcService.GetRegion(getRegionOptions)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error fetching region : %s: %s", config.Region, err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	// region check ends
	// resource group check
	if config.ResourceGroupID != "" && config.ResourceGroupName != "" {
		err := fmt.Errorf("[ERROR] Either one of resource_group_name or resource_group_id can be given, both together are not supported")
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	} else if config.ResourceGroupID != "" || config.ResourceGroupName != "" {
		rcUrl := config.RCEndpoint
		serviceClientOptions := &resourcemanagerv2.ResourceManagerV2Options{
			Authenticator: &core.IamAuthenticator{
				ApiKey: client.IBMApiKey,
				URL:    config.IAMEndpoint,
			},
			URL: rcUrl,
		}
		serviceClient, err := resourcemanagerv2.NewResourceManagerV2(serviceClientOptions)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error creating instance of ResourceManagerV2 for resource group: %s: %s", config.ResourceGroupID, err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		if config.ResourceGroupName != "" {
			reGrpName := resourcemanagerv2.ListResourceGroupsOptions{
				Name: &config.ResourceGroupName,
			}
			ResourceGroupName, _, errResNam := serviceClient.ListResourceGroups(&reGrpName)
			if errResNam != nil {
				err := fmt.Errorf("[ERROR] Error fetching resource group : %s: %s", config.ResourceGroupName, err)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
			if len(ResourceGroupName.Resources) == 1 {
				state.Put("derived_resource_group_id", *ResourceGroupName.Resources[0].ID)
			} else if len(ResourceGroupName.Resources) > 1 {
				id := *ResourceGroupName.Resources[0].ID
				state.Put("derived_resource_group_id", *ResourceGroupName.Resources[0].ID)
				ui.Say(fmt.Sprintf("[ERROR] Multiple resource group with the provided names found, using resource group with id: %s", id))
			} else {
				err := fmt.Errorf("[ERROR] Error fetching resource group, no resource group found with name : %s", config.ResourceGroupName)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
		} else {
			result, _, err := serviceClient.GetResourceGroup(serviceClient.NewGetResourceGroupOptions(config.ResourceGroupID))
			if err != nil {
				err := fmt.Errorf("[ERROR] Error fetching resource group : %s: %s", config.ResourceGroupID, err)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			} else if result == nil {
				err := fmt.Errorf("[ERROR] Resource group not found resource_group_id : %s: %s", config.ResourceGroupID, err)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
		}
	}

	// boot volume id validation
	if config.VSIBootVolumeID != "" {
		getVolumeOptions := &vpcv1.GetVolumeOptions{
			ID: &config.VSIBootVolumeID,
		}
		bootVolume, response, err := vpcService.GetVolume(getVolumeOptions)
		if err != nil {
			if response != nil && response.StatusCode == 404 {
				err := fmt.Errorf("[ERROR] Boot volume provided is not found : %s", config.VSIBootVolumeID)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
			err := fmt.Errorf("[ERROR] Error fetching volume %s", config.VSIBootVolumeID)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		if bootVolume.OperatingSystem == nil || bootVolume.OperatingSystem.Architecture == nil {
			err := fmt.Errorf("[ERROR] Provided volume %s is not a bootable volume. Please provide an unattached bootable volume", config.VSIBootVolumeID)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		if bootVolume.AttachmentState != nil && *bootVolume.AttachmentState != "unattached" {
			err := fmt.Errorf("[ERROR] Provided volume %s is either already attached or unusuble. Please provide an unattached bootable volume", config.VSIBootVolumeID)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}

	//boot snapshot support
	if config.VSIBootSnapshotID != "" {
		getSnapshotOptions := &vpcv1.GetSnapshotOptions{
			ID: &config.VSIBootSnapshotID,
		}
		bootSnapshot, response, err := vpcService.GetSnapshot(getSnapshotOptions)
		if err != nil {
			if response != nil && response.StatusCode == 404 {
				err := fmt.Errorf("[ERROR] Boot snapahot provided is not found %s:", config.VSIBootSnapshotID)
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
			err := fmt.Errorf("[ERROR] Error fetching snapshot %s", config.VSIBootSnapshotID)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		if bootSnapshot.OperatingSystem == nil || bootSnapshot.OperatingSystem.Architecture == nil {
			err := fmt.Errorf("[ERROR] Provided snapshot %s is not a bootable snapshot. Please provide an unattached bootable snapshot", config.VSIBootSnapshotID)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}

	// image check

	listImagesOptions := &vpcv1.ListImagesOptions{
		Name: &config.ImageName,
	}

	// if visibility != "" {
	// 	listImagesOptions.Visibility = &visibility
	// }
	availableImages, _, err := vpcService.ListImages(listImagesOptions)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error fetching custom image %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	allrecs := availableImages.Images

	if len(allrecs) != 0 {
		err := fmt.Errorf("[ERROR] An Image exist with the same name :%s", config.ImageName)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	// image check ends

	// usertags validation for blanks.
	if len(config.ImageTags) > 0 {
		for i := 0; i < len(config.ImageTags); i++ {
			if config.ImageTags[i] == "" {
				err := fmt.Errorf("[ERROR] Invalid user tag \"\", tags can be in `key:value` or `label` format, for example:, tags:\"my_tag\" ")
				state.Put("error", err)
				ui.Error(err.Error())
				return multistep.ActionHalt
			}
		}
	}

	// security group verification
	if config.SecurityGroupID != "" {
		secgrpOption := &vpcv1.GetSecurityGroupOptions{
			ID: &config.SecurityGroupID,
		}
		secGrp, _, err := vpcService.GetSecurityGroup(secgrpOption)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error fetching security group %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		if *secGrp.ID != "" {
			state.Put("user_sec_grp_vpc", *secGrp.VPC.ID) // check for vpc is done as part of subnet fetch.
		}
	}

	// crn validation

	if config.CatalogOfferingCRN != "" || config.CatalogOfferingVersionCRN != "" || config.EncryptionKeyCRN != "" {
		// validate crn

		searchURL := "https://api.global-search-tagging.cloud.ibm.com"
		globalSearchV2Options := &searchv2.GlobalSearchV2Options{
			URL:           searchURL,
			Authenticator: vpcService.Service.Options.Authenticator,
		}
		globalSearchAPIV2, err := searchv2.NewGlobalSearchV2(globalSearchV2Options)
		if err != nil {
			err = fmt.Errorf("[ERROR] Global Search service creation failed: %w", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		if action := verifyCRNs(globalSearchAPIV2, config, ui, state); action != multistep.ActionContinue {
			return action
		}
	}
	return multistep.ActionContinue
}

func (s *stepVerifyInput) Cleanup(state multistep.StateBag) {

}

// crnSearcher is the subset of *globalsearchv2.GlobalSearchV2 used to validate
// CRNs. It is declared as an interface so verifyCRNs can be exercised with a
// fake searcher in unit tests.
type crnSearcher interface {
	Search(searchOptions *searchv2.SearchOptions) (result *searchv2.ScanResult, response *core.DetailedResponse, err error)
}

// Compile-time guarantee that the production client satisfies crnSearcher, so an
// SDK signature change fails here rather than at the call site in Run.
var _ crnSearcher = (*searchv2.GlobalSearchV2)(nil)

// verifyCRNs confirms each configured CRN (catalog offering, catalog version,
// encryption key) resolves via Global Search. Any verification failure records
// the error under the "error" state key — which is how Packer core detects a
// failed build — and halts. Without that key the build would stop with no
// artifact yet exit 0 (a silent failure).
func verifyCRNs(search crnSearcher, config Config, ui packer.Ui, state multistep.StateBag) multistep.StepAction {
	checks := []struct {
		crn       string
		separator string
		// noun is the resource word in the success message ("Catalog",
		// "Encryption"); label is the longer phrase in the not-found message
		// ("Catalog crn", "Catalog version crn", "Encryption crn").
		noun  string
		label string
	}{
		{config.CatalogOfferingCRN, ":offering", "Catalog", "Catalog crn"},
		{config.CatalogOfferingVersionCRN, ":version", "Catalog", "Catalog version crn"},
		{config.EncryptionKeyCRN, ":key", "Encryption", "Encryption crn"},
	}
	for _, c := range checks {
		if c.crn == "" {
			continue
		}
		if action := verifyCRN(search, c.crn, c.separator, c.noun, c.label, ui, state); action != multistep.ActionContinue {
			return action
		}
	}
	return multistep.ActionContinue
}

// verifyCRN looks up a single CRN via Global Search. A Search transport/auth
// error, a nil result, or a result with no items are all treated as a failed
// verification: the error is stored under "error" and the step halts. A Search
// error must not be discarded — doing so previously turned an auth/network
// failure into either a nil-pointer panic or a misreported "not found".
func verifyCRN(search crnSearcher, crn, separator, noun, label string, ui packer.Ui, state multistep.StateBag) multistep.StepAction {
	crnToCheck := fmt.Sprintf("%s%s", strings.Split(crn, separator)[0], "::")
	query := fmt.Sprintf("crn:\"%s\"", crnToCheck)
	searchOptions := &searchv2.SearchOptions{
		Query: &query,
	}
	res, _, err := search.Search(searchOptions)
	if err != nil {
		err = fmt.Errorf("[ERROR] %s (%s) could not be verified via Global Search: %w", label, crn, err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	if res == nil || len(res.Items) == 0 {
		err := fmt.Errorf("[ERROR] %s (%s) information could not be retrieved", label, crn)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	ui.Say(fmt.Sprintf("%s %s information successfully retrieved ...", res.Items[0].GetProperty("name"), noun))
	return multistep.ActionContinue
}
