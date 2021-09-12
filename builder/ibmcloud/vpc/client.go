package vpc

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

//need to read about this http in go```
type IBMCloudClient struct {
	// // The http client for communicating
	http *http.Client

	// Credentials
	IBMApiKey string
}

type IBMCloudRequest struct {
	Parameters interface{} `json:"parameters"`
}

type InstanceType struct {
	EndPoint         string `json:",omitempty"`
	Version          string `json:",omitempty"`
	Generation       string `json:",omitempty"`
	Zone             string `json:",omitempty"`
	VPCID            string `json:",omitempty"`
	SubnetID         string
	ResourceGroupID  string
	VPCSSHKeyID      string `json:",omitempty"`
	VSIName          string
	VSIBaseImageID   string
	VSIBaseImageName string
	VSIProfile       string `json:",omitempty"`
	VSIInterface     string `json:",omitempty"`
	VSIUserDataFile  string `json:",omitempty"`
}

// IBM CLoud DataStructure to request VSI
type VPCInstanceReq struct {
	Name                    string            `json:"name"`
	Zone                    *ResourceByName   `json:"zone"`
	Vpc                     *ResourceByID     `json:"vpc"`
	PrimaryNetworkInterface *NetworkInterface `json:"primary_network_interface"`
	SSHKeys                 []*ResourceByID   `json:"keys"`
	Image                   *ResourceByID     `json:"image"`
	Profile                 *ResourceByName   `json:"profile"`
	VSIUserDataFile         string            `json:"user_data"`
	ResourceGroup           *ResourceByID     `json:"resource_group"`
}

type VPCSSHKeyRequest struct {
	Name          string        `json:"name"`
	PublicKey     string        `json:"public_key"`
	Type          string        `json:"type"`
	ResourceGroup *ResourceByID `json:"resource_group"`
}

type FloatingIPRequest struct {
	Name          string        `json:"name"`
	Target        *ResourceByID `json:"target"`
	ResourceGroup *ResourceByID `json:"resource_group"`
}

type SubnetRequest struct {
	Name                  string          `json:"name"`
	TotalIPv4AddressCount int             `json:"total_ipv4_address_count"`
	IPVersion             string          `json:"ip_version"`
	Zone                  *ResourceByName `json:"zone"`
	Vpc                   *ResourceByID   `json:"vpc"`
	ResourceGroup         *ResourceByID   `json:"resource_group"`
}

type SecurityGroupRequest struct {
	Name          string        `json:"name"`
	Vpc           *ResourceByID `json:"vpc"`
	ResourceGroup *ResourceByID `json:"resource_group"`
}

type SecurityGroupRuleRequest struct {
	Direction string `json:"direction"`
	Protocol  string `json:"protocol"`
	PortMin   int    `json:"port_min"`
	PortMax   int    `json:"port_max"`
	IpVersion string `json:"ip_version"`
}

type instanceActionRequest struct {
	Type string `json:"type"`
}

type SnapshotReq struct {
	Name          string        `json:"name"`
	SourceVolume  *ResourceByID `json:"source_volume"`
	ResourceGroup *ResourceByID `json:"resource_group"`
}

type ImageReq struct {
	Name          string        `json:"name"`
	SourceVolume  *ResourceByID `json:"source_volume"`
	ResourceGroup *ResourceByID `json:"resource_group"`
}

// ResourceByID - to create json by id
type ResourceByID struct {
	Id string `json:"id"`
}

// ResourceByName - to create json by name
type ResourceByName struct {
	Name string `json:"name"`
}

type NetworkInterface struct {
	Subnet *ResourceByID `json:"subnet"`
}

func (client IBMCloudClient) New(IBMApiKey string) *IBMCloudClient {
	return &IBMCloudClient{
		http: &http.Client{
			Transport: &http.Transport{
				Proxy: http.ProxyFromEnvironment,
			},
		},
		IBMApiKey: IBMApiKey,
	}
}

func (client IBMCloudClient) getIAMToken(state multistep.StateBag) error {
	ui := state.Get("ui").(packer.Ui)

	url := "https://iam.cloud.ibm.com/identity/token"
	var req *http.Request
	body := strings.NewReader(`grant_type=urn:ibm:params:oauth:grant-type:apikey&apikey=` + client.IBMApiKey)
	req, _ = http.NewRequest("POST", url, body)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.http.Do(req)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that generates the IAM token. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return err
	}
	defer resp.Body.Close()

	// Reading response
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed to get proper HTTP response from ibmcloud API. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return err
	}
	log.Println("Response Status - ", resp.StatusCode)

	// Unmarshal data so it can be accessed
	// For instance access id attribute inside the 'instanceData' JSON object
	// instanceId := fmt.Sprintf("'%s'", unmarshalData["id"])
	unmarshalData := make(map[string]interface{})
	errU := json.Unmarshal(responseBody, &unmarshalData)
	if errU != nil {
		err := fmt.Errorf("[ERROR] Failed to properly Unmarshal response. Error: %s", errU)
		ui.Error(err.Error())
		log.Println(err.Error())
		return err
	}

	IAMToken := unmarshalData["token_type"].(string) + " " + unmarshalData["access_token"].(string)
	state.Put("IAMToken", IAMToken)
	log.Println(fmt.Sprintf("IAM Access Token: %s", IAMToken))
	return nil
}

func (client IBMCloudClient) VPCCreateInstance(instance InstanceType, state multistep.StateBag) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)

	validName, err := regexp.Compile(`[^a-z0-9\-]+`)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error validating the Instance's name. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	instance.VSIName = validName.ReplaceAllString(instance.VSIName, "")

	// Read user_data_file
	var userData string
	userData = ""
	if instance.VSIUserDataFile != "" {
		fileData, err := ioutil.ReadFile(instance.VSIUserDataFile)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error reading `user_data_file`. Error: %s", err)
			ui.Error(err.Error())
			log.Println(err.Error())
			return nil, err
		}
		// Convert []byte to string
		userData = string(fileData)
	}

	// Construct the instance request object which will be decoded into json and posted to the API
	instanceRequest := &VPCInstanceReq{
		Name: instance.VSIName,
		Zone: &ResourceByName{
			Name: instance.Zone,
		},
		Vpc: &ResourceByID{
			Id: instance.VPCID,
		},
		PrimaryNetworkInterface: &NetworkInterface{
			Subnet: &ResourceByID{
				Id: instance.SubnetID,
			},
		},
		SSHKeys: []*ResourceByID{
			{
				Id: instance.VPCSSHKeyID,
			},
		},
		Image: &ResourceByID{
			Id: instance.VSIBaseImageID,
		},
		Profile: &ResourceByName{
			Name: instance.VSIProfile,
		},
		VSIUserDataFile: userData,
	}

	if instance.ResourceGroupID != "" {
		instanceRequest.ResourceGroup = &ResourceByID{
			Id: instance.ResourceGroupID,
		}
	}

	// Create payload
	payload, err := json.Marshal(instanceRequest)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating instance payload. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	// Create url
	url := client.newUrl("POST", "", "instances", "", "", state)
	// http request
	instanceData, err := client.newHttpRequest(url, payload, "POST", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that creates the instance. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return instanceData, nil
}

func (client IBMCloudClient) deleteResource(resourceID string, resourceType string, state multistep.StateBag) (string, error) {
	ui := state.Get("ui").(packer.Ui)
	// Create url
	url := client.newUrl("DELETE", resourceID, resourceType, "", "", state)

	var req *http.Request
	req, _ = http.NewRequest("DELETE", url, nil)

	var IAMToken string
	if state.Get("IAMToken") != nil {
		IAMToken = state.Get("IAMToken").(string)
	}
	req.Header.Set("Authorization", IAMToken)

	resp, err := client.http.Do(req)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that DELETE a resource. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "404", err
	}
	defer resp.Body.Close()

	return resp.Status, nil
}

// GET url --> EndPoint + "/" + resourceType + "/" + resourceID + parameters + query + "?" + Version + "&" + Generation
// Note slash before and after for parameters (parameters = "/blah/blah/blah/") and before for query (query="/blah")
func (client IBMCloudClient) newUrl(requestType string, resourceID string, resourceType string, parameters string, query string, state multistep.StateBag) string {
	config := state.Get("config").(Config)
	if requestType == "POST" {
		// "https://us-south.iaas.cloud.ibm.com/v1/instances?version=2020-08-11&generation=2"
		return config.EndPoint + resourceType + "?" + config.Version + "&" + config.Generation
	} else if requestType == "GET" || requestType == "DELETE" || requestType == "PUT" {
		// "https://us-south.iaas.cloud.ibm.com/v1/instances/$instance_id?version=2020-08-11&generation=2"
		return config.EndPoint + resourceType + "/" + resourceID + parameters + query + "?" + config.Version + "&" + config.Generation
	}
	return ""
}

func (client IBMCloudClient) newHttpRequest(url string, payload []byte, requestType string, state multistep.StateBag) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)
	var req *http.Request

	if requestType == "POST" {
		req, _ = http.NewRequest(requestType, url, strings.NewReader(string(payload)))
	} else if requestType == "GET" || requestType == "PUT" {
		req, _ = http.NewRequest(requestType, url, nil)
	}

	// Adding headers to the request
	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("Accept", "application/json")

	var IAMToken string
	if state.Get("IAMToken") != nil {
		IAMToken = state.Get("IAMToken").(string)
	}

	req.Header.Add("Authorization", IAMToken)

	resp, err := client.http.Do(req)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending a HTTP Request. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	defer resp.Body.Close()

	// Reading response
	responseBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed to get proper HTTP response from ibmcloud API. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	if resp.StatusCode == 400 {
		err := fmt.Errorf("[ERROR] Status 400: Bad Request - Response Body from ibmcloud: %s", string(responseBody))
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	if resp.StatusCode == 401 {
		msg := fmt.Errorf("[ERROR] Status 401: Unauthorized - The service token was expired or invalid: %s", string(responseBody))
		log.Println(msg.Error())

		ui.Say("The IAM Access Token was expired or invalid. Generating a new one...")
		err := client.getIAMToken(state)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error generating the IAM Access Token %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return nil, err
		}
		ui.Say("New IAM Access Token successfully generated!")

		// Re-Do the Request with the new token
		response, err := client.newHttpRequest(url, payload, requestType, state)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error: %s", err)
			ui.Error(err.Error())
			log.Println(err.Error())
			return nil, err
		}
		return response, err
	}

	log.Println("Response Status - ", resp.StatusCode)
	log.Println("Response Body from ibmcloud- ", string(responseBody))

	// Unmarshal data so it can be accessed: for instance access id attribute inside the 'unmarshalData' JSON object
	// instanceId := fmt.Sprintf("'%s'", unmarshalData["id"])
	unmarshalData := make(map[string]interface{})
	errU := json.Unmarshal(responseBody, &unmarshalData)
	if errU != nil {
		err := fmt.Errorf("[ERROR] Failed to properly Unmarshal response. Error: %s", errU)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	return unmarshalData, nil
}

func (client IBMCloudClient) waitForResourceReady(resourceID string, resourceType string, timeout time.Duration, state multistep.StateBag) error {
	ui := state.Get("ui").(packer.Ui)
	done := make(chan struct{})
	defer close(done)
	result := make(chan error, 1)

	go func() {
		attempts := 0
		for {
			attempts += 1
			if attempts%6 == 0 {
				ui.Say(fmt.Sprintf("Waiting time: %d minutes", attempts/6))
			} else {
				ui.Say(".")
			}

			log.Printf("Checking resource state... (attempt: %d)", attempts)
			isReady, err := client.isResourceReady(resourceID, resourceType, state)

			if err != nil {
				result <- err
				return
			}

			if isReady {
				result <- nil
				return
			}

			// Wait 10 seconds in between
			time.Sleep(10 * time.Second)

			// Verify we shouldn't exit
			select {
			case <-done:
				// We finished, so just exit the go routine
				return
			default:
				// Keep going
			}
		}
	}()

	log.Printf("Waiting for up to %d seconds for resource to become ready", timeout/time.Second)
	select {
	case err := <-result:
		return err
	case <-time.After(timeout):
		err := fmt.Errorf("timeout while waiting to for the resource to become ready")
		return err
	}
}

func (client IBMCloudClient) isResourceReady(resourceID string, resourceType string, state multistep.StateBag) (bool, error) {
	var ready bool
	if resourceType == "instances" {
		status, err := client.getStatus(resourceID, resourceType, state)
		ready = status == "running"
		return ready, err
	} else if resourceType == "floating_ips" || resourceType == "subnets" {
		status, err := client.getStatus(resourceID, resourceType, state)
		ready = status == "available"
		return ready, err
	} else if resourceType == "images" {
		status, err := client.getStatus(resourceID, resourceType, state)
		ready = status == "available"
		return ready, err
	}
	return ready, nil
}

func (client IBMCloudClient) getStatus(resourceID string, resourceType string, state multistep.StateBag) (string, error) {
	ui := state.Get("ui").(packer.Ui)
	url := client.newUrl("GET", resourceID, resourceType, "", "", state)
	response, err := client.newHttpRequest(url, nil, "GET", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed getting resource's status. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}
	return response["status"].(string), nil
}

func (client IBMCloudClient) waitForResourceDown(resourceID string, resourceType string, timeout time.Duration, state multistep.StateBag) error {
	ui := state.Get("ui").(packer.Ui)
	done := make(chan struct{})
	defer close(done)
	result := make(chan error, 1)

	go func() {
		attempts := 0
		for {
			attempts += 1
			if attempts%6 == 0 {
				ui.Say(fmt.Sprintf("Waiting time: %d minutes", attempts/6))
			} else {
				ui.Say(".")
			}

			log.Printf("Checking resource state... (attempt: %d)", attempts)
			isDown, err := client.isResourceDown(resourceID, resourceType, state)

			if err != nil {
				result <- err
				return
			}

			if isDown {
				result <- nil
				return
			}

			// Wait 10 seconds in between
			time.Sleep(10 * time.Second)

			// Verify we shouldn't exit
			select {
			case <-done:
				// We finished, so just exit the go routine
				return
			default:
				// Keep going
			}
		}
	}()

	log.Printf("Waiting for up to %d seconds for resource to be stopped", timeout/time.Second)
	select {
	case err := <-result:
		return err
	case <-time.After(timeout):
		err := fmt.Errorf("timeout while waiting to for the resource to be stopped")
		return err
	}
}

func (client IBMCloudClient) isResourceDown(resourceID string, resourceType string, state multistep.StateBag) (bool, error) {
	var down bool
	if resourceType == "instances" {
		status, err := client.getStatus(resourceID, resourceType, state)
		down = status == "stopped"
		return down, err
	}
	return down, nil
}

// Perfomr actions (stops, reboot, etc.) over an instance
func (client IBMCloudClient) manageInstance(resourceID string, resourceType string, action string, state multistep.StateBag) (string, error) {
	ui := state.Get("ui").(packer.Ui)
	instance := state.Get("instance_definition").(InstanceType)
	url := instance.EndPoint + resourceType + "/" + resourceID + "/actions?" + instance.Version + "&" + instance.Generation

	// Construct the Instance Action object which will be decoded into json and posted to the API
	instanceActionRequest := &instanceActionRequest{
		Type: action,
	}

	// Create Instance Action Payload
	payload, err := json.Marshal(instanceActionRequest)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed to perform %s action over instance. Error: %s", action, err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}

	response, err := client.newHttpRequest(url, payload, "POST", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed to perform %s action over instance. Error: %s", action, err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}

	if response["status"] == nil {
		return "", nil
	}
	return response["status"].(string), nil
}

func (client IBMCloudClient) retrieveResource(resourceID string, resourceType string, state multistep.StateBag) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)
	url := client.newUrl("GET", resourceID, resourceType, "", "", state)
	resourceData, err := client.newHttpRequest(url, nil, "GET", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed retrieving resource information. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return resourceData, nil
}

func (client IBMCloudClient) createFloatingIP(state multistep.StateBag) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)
	instanceData := state.Get("instance_data").(map[string]interface{})
	instanceResourceGroup := instanceData["resource_group"].(map[string]interface{})
	instanceResourceGroupID := instanceResourceGroup["id"].(string)

	networkInterfaces := instanceData["network_interfaces"].([]interface{})
	instanceNetworkInterface := networkInterfaces[0].(map[string]interface{})
	networkInterfaceID := instanceNetworkInterface["id"].(string)

	// Construct the Floating IP object which will be decoded into json and posted to the API
	floatingIPRequest := &FloatingIPRequest{
		Name: config.FloatingIPName,
	}
	floatingIPRequest.Target = &ResourceByID{
		Id: networkInterfaceID,
	}

	if config.ResourceGroupID != "" {
		floatingIPRequest.ResourceGroup = &ResourceByID{
			Id: instanceResourceGroupID,
		}
	}

	// Create Floating IP Payload
	payload, err := json.Marshal(floatingIPRequest)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed creating Floating IP Payload. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	url := client.newUrl("POST", "", "floating_ips", "", "", state)
	response, err := client.newHttpRequest(url, payload, "POST", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed creating Floating IP Request. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return response, err
}

func (client IBMCloudClient) GrabCredentials(instanceID string, state multistep.StateBag) (string, string, error) {
	ui := state.Get("ui").(packer.Ui)
	url := client.newUrl("GET", instanceID, "instances", "", "/initialization", state)
	instanceCredentials, _ := client.newHttpRequest(url, nil, "GET", state)
	password := instanceCredentials["password"].(map[string]interface{})
	encryptedPassword := password["encrypted_password"].(string)

	windowsPassword, err := client.DecryptPassword(encryptedPassword, state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed Grabbing Instance Credentials - Unable to obtain Windows' Password. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", "", err
	}

	return "Administrator", windowsPassword, nil
}

// Decrypt Password - Following documentation https://cloud.ibm.com/docs/vpc?topic=vpc-vsi_is_connecting_windows
func (client IBMCloudClient) DecryptPassword(encryptedPwd string, state multistep.StateBag) (string, error) {
	ui := state.Get("ui").(packer.Ui)
	///// Step 1: Decode the encrypted password
	decoded64Pwd, err := base64.StdEncoding.DecodeString(string(encryptedPwd))
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed Decoding the encrypted password. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}

	///// Step 2: Create working folder "data" and store decoded password on data/decoded_pwd.txt
	_ = os.Mkdir("data", 0755)
	file, err := os.Create("data/decoded_pwd.txt")
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed creating decoded password. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}
	file.Write(decoded64Pwd)
	file.Close()

	///// Step 3: Decrypt the decoded password using the RSA private key
	pathPrivateKey := state.Get("PRIVATE_KEY").(string)
	password, err := exec.Command("openssl", "pkeyutl", "-in", "data/decoded_pwd.txt", "-decrypt", "-inkey", pathPrivateKey).Output()
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed decrypting the decoded password. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}

	///// Step 4: Delete working dir
	defer os.RemoveAll("data")
	return string(password), err
}

func (client IBMCloudClient) createImage(state multistep.StateBag, imageData ImageReq) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)
	payload, err := json.Marshal(imageData)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating image payload. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	url := client.newUrl("POST", "", "images", "", "", state)
	response, err := client.newHttpRequest(url, payload, "POST", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that creates the image. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return response, nil
}

func (client IBMCloudClient) createSSHKeyVPC(state multistep.StateBag) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)

	file := state.Get("PUBLIC_KEY").(string)
	content, err := ioutil.ReadFile(file)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error reading SSH Public Key. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	publicKey := string(content)
	state.Put("ssh_public_key", publicKey)

	VpcSshKeyRequest := &VPCSSHKeyRequest{
		Name:      config.VpcSshKeyName,
		PublicKey: publicKey,
		Type:      "rsa",
	}

	if config.ResourceGroupID != "" {
		VpcSshKeyRequest.ResourceGroup = &ResourceByID{
			Id: config.ResourceGroupID,
		}
	}

	// Create VpcSshKey Payload
	payload, err := json.Marshal(VpcSshKeyRequest)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating SSH Key for VPC payload. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	url := client.newUrl("POST", "", "keys", "", "", state)
	response, err := client.newHttpRequest(url, payload, "POST", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that creates the SSH Key for VPC. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return response, nil
}

func (client IBMCloudClient) retrieveSubnet(state multistep.StateBag, subnetID string) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)
	url := client.newUrl("GET", subnetID, "subnets", "", "", state)
	response, err := client.newHttpRequest(url, nil, "GET", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error retrieving Subnet information. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return response, nil

}

func (client IBMCloudClient) createSecurityGroup(state multistep.StateBag, securityGroupData SecurityGroupRequest) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)
	payload, err := json.Marshal(securityGroupData)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating Security Group payload. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	url := client.newUrl("POST", "", "security_groups", "", "", state)
	response, err := client.newHttpRequest(url, payload, "POST", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that creates the Security Group. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return response, nil
}

func (client IBMCloudClient) createRule(SecurityGroupID string, rule SecurityGroupRuleRequest, state multistep.StateBag) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)
	payload, err := json.Marshal(rule)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating Security Group's rule payload. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	resourceType := "security_groups/" + SecurityGroupID + "/rules"
	url := client.newUrl("POST", "", resourceType, "", "", state)
	response, err := client.newHttpRequest(url, payload, "POST", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that creates a Security Group's rule. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return response, nil
}

func (client IBMCloudClient) addNetworkInterfaceToSecurityGroup(SecurityGroupID string, networkInterfaceID string, state multistep.StateBag) (map[string]interface{}, error) {
	ui := state.Get("ui").(packer.Ui)
	resourceType := "security_groups/" + SecurityGroupID + "/network_interfaces"
	url := client.newUrl("PUT", networkInterfaceID, resourceType, "", "", state)
	response, err := client.newHttpRequest(url, nil, "PUT", state)

	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that Add the VSI's network interface to the Security Group. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return response, nil
}

func (client IBMCloudClient) getImageIDByName(name string, state multistep.StateBag) (string, error) {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)

	validName, err := regexp.Compile(`[^a-z0-9\-]+`)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error validating the image's name. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}
	name = validName.ReplaceAllString(name, "")

	url := config.EndPoint + "/" + "images" + "?" + "name=" + name + "&" + config.Version + "&" + config.Generation
	response, err := client.newHttpRequest(url, nil, "GET", state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that get the Images. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}
	return response["images"].([]interface{})[0].(map[string]interface{})["id"].(string), nil
}
