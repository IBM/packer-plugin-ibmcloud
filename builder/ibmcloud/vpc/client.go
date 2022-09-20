package vpc

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

type IBMCloudClient struct {
	// // The http client for communicating
	http *http.Client

	// Credentials
	IBMApiKey string
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
	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	if resourceType == "instances" {
		options := vpcService.NewGetInstanceOptions(resourceID)
		instance, _, err := vpcService.GetInstance(options)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error occurred while getting instance information. Error: %s", err)
			return false, fmt.Errorf(err.Error())
		}
		status := *instance.Status
		if status == "failed" {
			err := fmt.Errorf("[ERROR] Instance return with failed status. Status Reason - %s: %s", status, *instance.StatusReasons[0].Message)
			return false, fmt.Errorf(err.Error())
		}
		ready = status == "running"
		return ready, err
	} else if resourceType == "floating_ips" {
		options := vpcService.NewGetFloatingIPOptions(resourceID)
		floatingIP, _, err := vpcService.GetFloatingIP(options)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error occurred while getting floating ip information. Error: %s", err)
			return false, fmt.Errorf(err.Error())
		}
		status := *floatingIP.Status
		ready = status == "available"
		return ready, err
	} else if resourceType == "subnets" {
		options := vpcService.NewGetSubnetOptions(resourceID)
		subnet, _, err := vpcService.GetSubnet(options)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error occurred while getting subnet information. Error: %s", err)
			return false, fmt.Errorf(err.Error())
		}
		status := *subnet.Status
		ready = status == "available"
		return ready, err
	} else if resourceType == "images" {
		options := vpcService.NewGetImageOptions(resourceID)
		image, _, err := vpcService.GetImage(options)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error occurred while getting image information. Error: %s", err)
			return false, fmt.Errorf(err.Error())
		}
		status := *image.Status
		ready = status == "available"
		return ready, err
	}
	return ready, nil
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

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}
	if resourceType == "instances" {
		options := &vpcv1.GetInstanceOptions{}
		options.SetID(resourceID)
		instance, _, err := vpcService.GetInstance(options)
		status := *instance.Status
		down = status == "stopped"
		return down, err
	}
	return down, nil
}

// Perfomr actions (stops, reboot, etc.) over an instance
func (client IBMCloudClient) manageInstance(resourceID string, action string, state multistep.StateBag) (string, error) {
	ui := state.Get("ui").(packer.Ui)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	// Construct the Instance Action object which will be decoded into json and posted to the API
	// Create Instance Action Payload
	options := &vpcv1.CreateInstanceActionOptions{}
	options.SetInstanceID(resourceID)
	options.SetType(action)
	response, _, err := vpcService.CreateInstanceAction(options)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed to perform %s action over instance. Error: %s", action, err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}

	if response.Status == nil {
		return "", nil
	}
	return *response.Status, nil
}

func (client IBMCloudClient) retrieveResource(resourceID string, state multistep.StateBag) (*vpcv1.Instance, error) {
	ui := state.Get("ui").(packer.Ui)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}
	options := &vpcv1.GetInstanceOptions{}
	options.SetID(resourceID)
	instance, _, err := vpcService.GetInstance(options)

	if err != nil {
		err := fmt.Errorf("[ERROR] Failed retrieving resource information. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return instance, nil
}

func (client IBMCloudClient) createFloatingIP(state multistep.StateBag) (*vpcv1.FloatingIP, error) {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	instanceData := state.Get("instance_data").(*vpcv1.Instance)
	instanceResourceGroup := instanceData.ResourceGroup
	instanceResourceGroupID := *instanceResourceGroup.ID

	networkInterfaces := instanceData.NetworkInterfaces
	instanceNetworkInterface := networkInterfaces[0]
	networkInterfaceID := *instanceNetworkInterface.ID

	options := &vpcv1.CreateFloatingIPOptions{}
	options.SetFloatingIPPrototype(&vpcv1.FloatingIPPrototype{
		Name: &config.FloatingIPName,
		Target: &vpcv1.FloatingIPByTargetNetworkInterfaceIdentityNetworkInterfaceIdentityByID{
			ID: &networkInterfaceID,
		},
		ResourceGroup: &vpcv1.ResourceGroupIdentityByID{
			ID: &instanceResourceGroupID,
		},
	})
	floatingIP, _, err := vpcService.CreateFloatingIP(options)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed creating Floating IP Request. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return floatingIP, err
}

func (client IBMCloudClient) GrabCredentials(instanceID string, state multistep.StateBag) (string, string, error) {
	ui := state.Get("ui").(packer.Ui)
	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}
	options := &vpcv1.GetInstanceInitializationOptions{
		ID: &instanceID,
	}
	instanceCredentials, _, err := vpcService.GetInstanceInitialization(options)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed getting instance initialization data. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", "", err
	}
	password := *instanceCredentials.Password.EncryptedPassword
	windowsPassword, err := client.DecryptPassword(password, state)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed Grabbing Instance Credentials - Unable to obtain Windows' Password. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", "", err
	}

	return "Administrator", windowsPassword, nil
}

// Decrypt Password - Following documentation https://cloud.ibm.com/docs/vpc?topic=vpc-vsi_is_connecting_windows
func (client IBMCloudClient) DecryptPassword(encryptedPwd []byte, state multistep.StateBag) (string, error) {
	ui := state.Get("ui").(packer.Ui)

	///// Step 1: Create working folder "data" and store bas64 password on data/decoded_pwd.txt
	_ = os.Mkdir("data", 0755)
	file, err := os.Create("data/decoded_pwd.txt")
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed writing decoded password. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}
	file.Write(encryptedPwd)
	file.Close()

	///// Step 2: Decrypt the decoded password using the RSA private key
	pathPrivateKey := state.Get("PRIVATE_KEY").(string)
	password, err := exec.Command("openssl", "pkeyutl", "-in", "data/decoded_pwd.txt", "-decrypt", "-inkey", pathPrivateKey).Output()
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed decrypting the decoded password. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return "", err
	}

	///// Step 3: Delete working dir
	defer os.RemoveAll("data")
	return string(password), err
}

func (client IBMCloudClient) createSSHKeyVPC(state multistep.StateBag) (*vpcv1.Key, error) {
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

	options := &vpcv1.CreateKeyOptions{}
	options.SetName(config.VpcSshKeyName)
	options.SetPublicKey(publicKey)
	if config.ResourceGroupID != "" {
		options.SetResourceGroup(
			&vpcv1.ResourceGroupIdentity{
				ID: &config.ResourceGroupID,
			},
		)
	}
	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	key, _, err := vpcService.CreateKey(options)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that creates the SSH Key for VPC. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return key, nil
}
func (client IBMCloudClient) getSecurityGroup(state multistep.StateBag, securityGroupData vpcv1.GetSecurityGroupOptions) (*vpcv1.SecurityGroup, error) {
	ui := state.Get("ui").(packer.Ui)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	securityGroup, _, err := vpcService.GetSecurityGroup(&securityGroupData)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error getting the Security Group. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return securityGroup, nil
}
func (client IBMCloudClient) createSecurityGroup(state multistep.StateBag, securityGroupData vpcv1.CreateSecurityGroupOptions) (*vpcv1.SecurityGroup, error) {
	ui := state.Get("ui").(packer.Ui)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	securityGroup, _, err := vpcService.CreateSecurityGroup(&securityGroupData)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error creating the Security Group. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return securityGroup, nil
}

func (client IBMCloudClient) createRule(rule vpcv1.CreateSecurityGroupRuleOptions, state multistep.StateBag) (*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp, error) {
	ui := state.Get("ui").(packer.Ui)

	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}

	securityGroupRuleIntf, _, err := vpcService.CreateSecurityGroupRule(&rule)
	securityGroupRule := securityGroupRuleIntf.(*vpcv1.SecurityGroupRuleSecurityGroupRuleProtocolTcpudp)

	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that creates a Security Group's rule. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	return securityGroupRule, nil
}

func (client IBMCloudClient) addNetworkInterfaceToSecurityGroup(securityGroupID string, networkInterfaceID string, state multistep.StateBag) (*vpcv1.SecurityGroupTargetReference, error) {
	ui := state.Get("ui").(packer.Ui)
	var vpcService *vpcv1.VpcV1
	if state.Get("vpcService") != nil {
		vpcService = state.Get("vpcService").(*vpcv1.VpcV1)
	}
	options := vpcService.NewCreateSecurityGroupTargetBindingOptions(
		securityGroupID,
		networkInterfaceID,
	)
	securityGroupTargetReferenceIntf, _, err := vpcService.CreateSecurityGroupTargetBinding(options)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error sending the HTTP request that Add the VSI's network interface to the Security Group. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}
	securityGroupTargetReference := securityGroupTargetReferenceIntf.(*vpcv1.SecurityGroupTargetReference)

	return securityGroupTargetReference, nil
}
