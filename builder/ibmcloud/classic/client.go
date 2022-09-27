package classic

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
)

const SOFTLAYER_API_URL = "api.softlayer.com/rest/v3"

type SoftlayerClient struct {
	// The http client for communicating
	http *http.Client

	// Credentials
	user   string
	apiKey string
}

type SoftLayerRequest struct {
	Parameters interface{} `json:"parameters"`
}

// Based on: http://sldn.softlayer.com/reference/datatypes/SoftLayer_Container_Virtual_Guest_Configuration/
type InstanceType struct {
	HostName     string `json:"hostname"`
	Domain       string
	Datacenter   string
	PublicVlanId int64

	Flavor       string `json:",omitempty"`
	Cpus         int    `json:",omitempty"`
	Memory       int64  `json:",omitempty"`
	DiskCapacity int    `json:",omitempty"`

	HourlyBillingFlag      bool
	LocalDiskFlag          bool
	NetworkSpeed           int
	ProvisioningSshKeyId   int64
	BaseImageId            string
	BaseOsCode             string
	PublicSecurityGroupIds []int64
	UserData               []Attribute
	UserDataCount          uint64
}

type InstanceReq struct {
	HostName     string                   `json:"hostname"`
	Domain       string                   `json:"domain"`
	Datacenter   *Datacenter              `json:"datacenter"`
	PublicVlanId *PrimaryNetworkComponent `json:"primaryNetworkComponent,omitempty"`

	SupplementalCreateObjectOptions *SupplementalCreateObjectOptions `json:"supplementalCreateObjectOptions,omitempty"`
	Cpus                            int                              `json:"startCpus,omitempty"`
	Memory                          int64                            `json:"maxMemory,omitempty"`
	BlockDevices                    []*BlockDevice                   `json:"blockDevices,omitempty"`

	HourlyBillingFlag        bool                      `json:"hourlyBillingFlag"`
	LocalDiskFlag            bool                      `json:"localDiskFlag"`
	NetworkComponents        []*NetworkComponent       `json:"networkComponents"`
	BlockDeviceTemplateGroup *BlockDeviceTemplateGroup `json:"blockDeviceTemplateGroup,omitempty"`
	OsReferenceCode          string                    `json:"operatingSystemReferenceCode,omitempty"`
	SshKeys                  []*SshKey                 `json:"sshKeys,omitempty"`
	UserData                 []Attribute               `json:"userData,omitempty"`
	UserDataCount            uint64                    `json:"userDataCount,omitempty"`
}

type InstanceImage struct {
	Descption string `json:"description"`
	Name      string `json:"name"`
	Summary   string `json:"summary"`
}

type SupplementalCreateObjectOptions struct {
	Flavor string `json:"flavorKeyName"`
}

type ImageDatacenters struct {
	Id string `json:"id"`
}

type Datacenter struct {
	Name string `json:"name"`
}

type NetworkComponent struct {
	MaxSpeed int `json:"maxSpeed"`
}

type BlockDeviceTemplateGroup struct {
	Id string `json:"globalIdentifier"`
}

type DiskImage struct {
	Capacity int `json:"capacity"`
}

type SshKey struct {
	Id    int64  `json:"id,omitempty"`
	Key   string `json:"key,omitempty"`
	Label string `json:"label,omitempty"`
}

type BlockDevice struct {
	Id        int64      `json:"id,omitempty"`
	Device    string     `json:"device,omitempty"`
	DiskImage *DiskImage `json:"diskImage,omitempty"`
}

type PrimaryNetworkComponent struct {
	SecurityGroupBindings []*SecurityGroupBindings `json:"securityGroupBindings,omitempty"`
	NetworkVlan           *NetworkVlan             `json:"networkVlan,omitempty"`
}
type NetworkVlan struct {
	Id int64 `json:"id,omitempty"`
}

type SecurityGroupBindings struct {
	SecurityGroup *SecurityGroup `json:"securityGroup,omitempty"`
}

type SecurityGroup struct {
	Id int64 `json:"id,omitempty"`
}

type Attribute struct {
	Value string `json:"value,omitempty"`
}

func (s SoftlayerClient) New(user string, key string) *SoftlayerClient {
	return &SoftlayerClient{
		http: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
		user:   user,
		apiKey: key,
	}
}

func (s SoftlayerClient) generateRequestBody(params ...interface{}) (*bytes.Buffer, error) {
	softlayerRequest := &SoftLayerRequest{
		Parameters: params,
	}

	log.Printf("SoftLayerRequest: %s", softlayerRequest)
	body, err := json.Marshal(softlayerRequest)
	if err != nil {
		return nil, err
	}

	log.Printf("Generated a request: %s", body)

	return bytes.NewBuffer(body), nil
}

func (s SoftlayerClient) hasErrors(body map[string]interface{}) error {
	if errString, ok := body["error"]; !ok {
		return nil
	} else {
		return errors.New(errString.(string))
	}
}

func (s SoftlayerClient) doRawHttpRequest(path string, requestType string, requestBody *bytes.Buffer) ([]byte, error) {
	url := fmt.Sprintf("https://%s:%s@%s/%s", s.user, s.apiKey, SOFTLAYER_API_URL, path)
	log.Printf("Sending new request to softlayer: %s", url)

	// Create the request object
	var lastResponse http.Response
	switch requestType {
	case "POST", "DELETE":
		req, err := http.NewRequest(requestType, url, requestBody)

		if err != nil {
			return nil, err
		}
		resp, err := s.http.Do(req)

		if err != nil {
			return nil, err
		} else {
			lastResponse = *resp
		}
	case "GET":
		resp, err := http.Get(url)

		if err != nil {
			return nil, err
		} else {
			lastResponse = *resp
		}
	default:
		return nil, fmt.Errorf("[ERROR] Undefined request type '%s', only GET/POST/DELETE are available", requestType)
	}

	responseBody, err := ioutil.ReadAll(lastResponse.Body)
	lastResponse.Body.Close()
	if err != nil {
		return nil, err
	}

	log.Printf("Received response from SoftLayer: %s", responseBody)
	return responseBody, nil
}

func (s SoftlayerClient) doHttpRequest(path string, requestType string, requestBody *bytes.Buffer) ([]interface{}, error) {
	responseBody, err := s.doRawHttpRequest(path, requestType, requestBody)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed to get proper HTTP response from SoftLayer API. Error: %s", err)
		return nil, err
	}
	log.Printf("ResponseBody: %s", responseBody)

	var decodedResponse interface{}
	err = json.Unmarshal(responseBody, &decodedResponse)
	if err != nil {
		err := fmt.Errorf("[Error] Failed to decode JSON response from SoftLayer: %s | %s", responseBody, err)
		return nil, err
	}

	switch v := decodedResponse.(type) {
	case []interface{}:
		return v, nil
	case map[string]interface{}:
		if err := s.hasErrors(v); err != nil {
			return nil, err
		}

		return []interface{}{v}, nil

	case nil:
		return []interface{}{nil}, nil
	default:
		return nil, errors.New("unexpected type in HTTP response")
	}
}

// HTTP response == nil

func (s SoftlayerClient) doModifiedHttpRequest(path string, requestType string, requestBody *bytes.Buffer) ([]interface{}, error) {
	responseBody, err := s.doRawHttpRequest(path, requestType, requestBody)
	if err != nil {
		err := fmt.Errorf("[Error] Failed to get proper HTTP response from SoftLayer API")
		return nil, err
	}
	log.Printf("ResponseBody: %s", responseBody)

	var decodedResponse interface{}
	err = json.Unmarshal(responseBody, &decodedResponse)
	if err != nil {
		err := fmt.Errorf("[Error] Failed to decode JSON response from SoftLayer: %s | %s", responseBody, err)
		return nil, err
	}
	log.Printf("response Unmarshal: %s", err)

	switch v := decodedResponse.(type) {
	case []interface{}:
		return v, nil
	case map[string]interface{}:
		if err := s.hasErrors(v); err != nil {
			return nil, err
		}

		return []interface{}{v}, nil

	case nil:
		return []interface{}{nil}, nil
	default:
		//return nil, errors.New("Unexpected type in HTTP response")
		return []interface{}{v}, nil
	}
}

func (s SoftlayerClient) CreateInstance(instance InstanceType) (map[string]interface{}, error) {
	// SoftLayer API puts some limitations on hostname and domain fields of the request
	validName, err := regexp.Compile(`[^A-Za-z0-9\-\.]+`)
	if err != nil {
		return nil, err
	}

	instance.HostName = validName.ReplaceAllString(instance.HostName, "")
	instance.Domain = validName.ReplaceAllString(instance.Domain, "")

	// Construct the instance request object which will be decoded into json and posted to the API
	instanceRequest := &InstanceReq{
		HostName: instance.HostName,
		Domain:   instance.Domain,
		Datacenter: &Datacenter{
			Name: instance.Datacenter,
		},

		Cpus:   instance.Cpus,
		Memory: instance.Memory,

		HourlyBillingFlag: instance.HourlyBillingFlag,
		LocalDiskFlag:     instance.LocalDiskFlag,
		NetworkComponents: []*NetworkComponent{
			{
				MaxSpeed: instance.NetworkSpeed,
			},
		},
		UserData:      instance.UserData,
		UserDataCount: instance.UserDataCount,
	}

	if instance.ProvisioningSshKeyId != 0 {
		instanceRequest.SshKeys = []*SshKey{
			{
				Id: instance.ProvisioningSshKeyId,
			},
		}
	}

	if instance.BaseImageId != "" {
		instanceRequest.BlockDeviceTemplateGroup = &BlockDeviceTemplateGroup{
			Id: instance.BaseImageId,
		}
	} else {
		instanceRequest.OsReferenceCode = instance.BaseOsCode

		if instance.Flavor == "" {
			instanceRequest.BlockDevices = []*BlockDevice{
				{
					Device: "0",
					DiskImage: &DiskImage{
						Capacity: instance.DiskCapacity,
					},
				},
			}
		}
	}

	if len(instance.PublicSecurityGroupIds) != 0 {
		var securityGroupList []*SecurityGroupBindings

		for i := 0; i < len(instance.PublicSecurityGroupIds); i++ {
			securityGroup := &SecurityGroupBindings{
				SecurityGroup: &SecurityGroup{
					Id: instance.PublicSecurityGroupIds[i],
				},
			}
			securityGroupList = append(securityGroupList, securityGroup)
		}

		if instance.PublicVlanId != 0 {
			instanceRequest.PublicVlanId = &PrimaryNetworkComponent{
				NetworkVlan: &NetworkVlan{
					Id: instance.PublicVlanId,
				},
				SecurityGroupBindings: securityGroupList,
			}
		}

	}

	if instance.Flavor != "" {
		instanceRequest.SupplementalCreateObjectOptions = &SupplementalCreateObjectOptions{
			Flavor: instance.Flavor,
		}
	}

	requestBody, err := s.generateRequestBody(instanceRequest)
	if err != nil {
		return nil, err
	}

	data, err := s.doHttpRequest("SoftLayer_Virtual_Guest/createObject.json", "POST", requestBody)
	if err != nil {
		return nil, err
	}

	return data[0].(map[string]interface{}), err
}

func (s SoftlayerClient) GrabCredentials(instanceID string, state multistep.StateBag) (string, string, error) {
	//ui := state.Get("ui").(packer.Ui)
	waitDuration := 10 * time.Second
	attemptCount := 60
	var str string
	var result map[string]map[string]map[string]interface{}

	for i := 0; i < attemptCount; i++ {
		data, err := s.doRawHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest/%s/getObject.json?objectMask=mask[id,operatingSystem[id,passwords[username,password]]]", instanceID), "GET", nil)
		//ui.Say(string(data))
		str = strings.ReplaceAll(string(data), "[", "")
		str = strings.ReplaceAll(str, "]", "")
		//ui.Say(str)
		json.Unmarshal([]byte(str), &result)
		//ui.Say(fmt.Sprintf("%v", result["operatingSystem"]["passwords"]["username"]))
		if err != nil {
			return "", "", err
		}
		if fmt.Sprintf("%v", result["operatingSystem"]["passwords"]["password"]) != "" {
			//ui.Say(fmt.Sprintf("Found password on attempt %v", i))
			return fmt.Sprintf("%v", result["operatingSystem"]["passwords"]["username"]), fmt.Sprintf("%v", result["operatingSystem"]["passwords"]["password"]), nil
		}
		time.Sleep(waitDuration)
	}
	return "", "", fmt.Errorf("[ERROR] Unable to obtain password after %v seconds", int(waitDuration)*int(attemptCount))
}

func (s SoftlayerClient) DestroyInstance(instanceId string) error {
	response, err := s.doRawHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest/%s.json", instanceId), "DELETE", new(bytes.Buffer))

	log.Printf("Deleted an Instance with id (%s), response: %s", instanceId, response)

	if res := string(response[:]); res != "true" {
		return fmt.Errorf("[ERROR] Failed to destroy and instance wit id '%s', got '%s' as response from the API", instanceId, res)
	}

	return err
}

func (s SoftlayerClient) UploadSshKey(label string, publicKey string) (keyId int64, err error) {
	sshKeyRequest := &SshKey{
		Key:   publicKey,
		Label: label,
	}

	requestBody, err := s.generateRequestBody(sshKeyRequest)
	if err != nil {
		return 0, err
	}

	data, err := s.doHttpRequest("SoftLayer_Security_Ssh_Key/createObject.json", "POST", requestBody)
	if err != nil {
		return 0, err
	}

	return int64(data[0].(map[string]interface{})["id"].(float64)), err
}

func (s SoftlayerClient) DestroySshKey(keyId int64) error {
	response, err := s.doRawHttpRequest(fmt.Sprintf("SoftLayer_Security_Ssh_Key/%v.json", int(keyId)), "DELETE", new(bytes.Buffer))

	log.Printf("Deleted an SSH Key with id (%v), response: %s", keyId, response)
	if res := string(response[:]); res != "true" {
		return fmt.Errorf("[ERROR] Failed to destroy and SSH key wit id '%v', got '%s' as response from the API", keyId, res)
	}

	return err
}

func (s SoftlayerClient) getInstancePublicIp(instanceId string) (string, error) {
	response, err := s.doRawHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest/%s/getPrimaryIpAddress.json", instanceId), "GET", nil)
	if err != nil {
		return "", nil
	}

	var validIp = regexp.MustCompile(`[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,4}\.[0-9]{1,4}`)
	ipAddress := validIp.Find(response)

	return string(ipAddress), nil
}

func (s SoftlayerClient) getBlockDevices(instanceId string) ([]interface{}, error) {
	data, err := s.doHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest/%s/getBlockDevices.json?objectMask=mask.diskImage.name", instanceId), "GET", nil)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s SoftlayerClient) findNonSwapBlockDeviceIds(blockDevices []interface{}) []int64 {
	blockDeviceIds := make([]int64, len(blockDevices))
	deviceCount := 0

	for _, val := range blockDevices {
		blockDevice := val.(map[string]interface{})
		diskImage := blockDevice["diskImage"].(map[string]interface{})
		name := diskImage["name"].(string)
		id := int64(blockDevice["id"].(float64))

		if !strings.Contains(name, "SWAP") && !strings.Contains(name, "METADATA") {
			blockDeviceIds[deviceCount] = id
			deviceCount++
		}
	}

	return blockDeviceIds[:deviceCount]
}

func (s SoftlayerClient) getBlockDeviceTemplateGroups() ([]interface{}, error) {
	data, err := s.doHttpRequest("SoftLayer_Account/getBlockDeviceTemplateGroups.json", "GET", nil)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (s SoftlayerClient) findImageIdByName(imageName string) (string, error) {
	// Find the image id by listing all images and matching on name.
	var imageId string

	images, err := s.getBlockDeviceTemplateGroups()
	if err != nil {
		return "", err
	}

	for _, val := range images {
		image := val.(map[string]interface{})
		if image["name"] == imageName && image["globalIdentifier"] != nil {
			imageId = image["globalIdentifier"].(string)
			//imageNonGlobalId = image["id"].(string)
			break
		}
	}

	if imageId == "" {
		err = fmt.Errorf("no image found with name '%s'", imageName)
		return "", err
	}

	return imageId, nil
}

func (s SoftlayerClient) captureStandardImage(instanceId string, imageName string, imageDescription string, blockDeviceIds []int64) (map[string]interface{}, error) {
	blockDevices := make([]*BlockDevice, len(blockDeviceIds))
	for i, id := range blockDeviceIds {
		blockDevices[i] = &BlockDevice{
			Id: id,
		}
	}

	requestBody, err := s.generateRequestBody(imageName, blockDevices, imageDescription)
	if err != nil {
		return nil, err
	}

	data, err := s.doHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest/%s/createArchiveTransaction.json", instanceId), "POST", requestBody)
	if err != nil {
		return nil, err
	}

	return data[0].(map[string]interface{}), err
}

func (s SoftlayerClient) captureImage(instanceId string, imageName string, imageDescription string) (map[string]interface{}, error) {
	imageRequest := &InstanceImage{
		Descption: imageDescription,
		Name:      imageName,
		Summary:   imageDescription,
	}

	requestBody, err := s.generateRequestBody(imageRequest)
	if err != nil {
		return nil, err
	}

	data, err := s.doHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest/%s/captureImage.json", instanceId), "POST", requestBody)
	if err != nil {
		return nil, err
	}

	return data[0].(map[string]interface{}), err
}

func (s SoftlayerClient) destroyImage(imageId string) error {
	response, err := s.doRawHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest/%s.json", imageId), "DELETE", new(bytes.Buffer))

	log.Printf("Deleted an image with id (%s), response: %s", imageId, response)
	if res := string(response[:]); res != "true" {
		return fmt.Errorf("[Error] Failed to destroy and image wit id '%s', got '%s' as response from the API", imageId, res)
	}

	return err
}

//
func (s SoftlayerClient) copyImageToDatacenters(imageId string, datacenterId []string) error {

	var addLocationsList []interface{}

	for i := 0; i < len(datacenterId); i++ {

		addLocations := &ImageDatacenters{
			Id: datacenterId[i],
		}
		addLocationsList = append(addLocationsList, addLocations)
	}

	requestBody, err := s.generateRequestBody(addLocationsList)
	if err != nil {
		return err
	}

	data, err := s.doModifiedHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest_Block_Device_Template_Group/%s/addLocations.json", imageId), "POST", requestBody)
	if err != nil {
		return err
	}

	log.Printf("Copied image with id (%s) status: %s", imageId, data)
	return nil

}

func (s SoftlayerClient) isInstanceReady(instanceId string) (bool, error) {
	powerData, err := s.doHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest/%s/getPowerState.json", instanceId), "GET", nil)
	if err != nil {
		return false, nil
	}
	isPowerOn := powerData[0].(map[string]interface{})["keyName"].(string) == "RUNNING"

	transactionData, err := s.doHttpRequest(fmt.Sprintf("SoftLayer_Virtual_Guest/%s/getActiveTransaction.json", instanceId), "GET", nil)
	if err != nil {
		return false, nil
	}
	noTransactions := transactionData[0] == nil

	return isPowerOn && noTransactions, err
}

func (s SoftlayerClient) waitForInstanceReady(instanceId string, timeout time.Duration) error {
	done := make(chan struct{})
	defer close(done)
	result := make(chan error, 1)

	go func() {
		attempts := 0
		for {
			attempts += 1

			log.Printf("Checking instance status... (attempt: %d)", attempts)
			isReady, err := s.isInstanceReady(instanceId)
			if err != nil {
				result <- err
				return
			}

			if isReady {
				result <- nil
				return
			}

			// Wait 3 seconds in between
			time.Sleep(3 * time.Second)

			// Verify we shouldn't exit
			select {
			case <-done:
				// We finished, so just exit the goroutine
				return
			default:
				// Keep going
			}
		}
	}()

	log.Printf("Waiting for up to %d seconds for instance to become ready", timeout/time.Second)
	select {
	case err := <-result:
		return err
	case <-time.After(timeout):
		err := fmt.Errorf("timeout while waiting to for the instance to become ready")
		return err
	}
}
