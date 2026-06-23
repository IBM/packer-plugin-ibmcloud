package vpc

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/IBM/go-sdk-core/v5/core"
	"github.com/IBM/vpc-go-sdk/vpcv1"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// maxConsecutiveTransientPollFailures bounds how many consecutive transient
// errors (5xx/429 responses or network-level failures) a status-poll loop will
// tolerate before giving up. This keeps a genuinely unhealthy API from spinning
// until the overall StateTimeout while still riding out the occasional blip
// during a long-running bake. It applies to every poll loop in this package
// (pollUntil and the image-export poll).
const maxConsecutiveTransientPollFailures = 5

// defaultPollInterval is the wait between status polls when a caller does not
// set one explicitly.
const defaultPollInterval = 10 * time.Second

// transientPollError wraps an error from a single resource-status poll that is
// considered transient (a retryable 5xx/429 response or a network-level
// failure) and therefore safe to retry rather than fail the build outright.
type transientPollError struct {
	err error
}

func (e *transientPollError) Error() string { return e.err.Error() }
func (e *transientPollError) Unwrap() error { return e.err }

// isTransientPollError reports whether an error returned by an IBM VPC status
// call should be retried. resp may be nil when the request never reached the
// server (network timeout, connection reset, EOF), which is itself transient.
func isTransientPollError(resp *core.DetailedResponse, err error) bool {
	if err == nil {
		return false
	}
	if resp != nil && resp.StatusCode != 0 {
		switch resp.StatusCode {
		case http.StatusInternalServerError, // 500
			http.StatusBadGateway,         // 502
			http.StatusServiceUnavailable, // 503
			http.StatusGatewayTimeout,     // 504
			http.StatusTooManyRequests:    // 429
			return true
		default:
			// Any other response carrying a status code (e.g. 4xx) is fatal.
			return false
		}
	}
	// No HTTP response with a usable status code means the request failed at
	// the network level before the server answered. Treat that as transient.
	return true
}

// classifyPollError returns wrapped enclosed in a *transientPollError when the
// underlying failure is retryable, and wrapped unchanged when it is fatal. This
// lets the poll loops distinguish "retry" from "abort the build".
func classifyPollError(resp *core.DetailedResponse, err error, wrapped error) error {
	if isTransientPollError(resp, err) {
		return &transientPollError{err: wrapped}
	}
	return wrapped
}

// sleepOrDone waits interval between polls and then reports whether the poll
// goroutine should stop because its parent has already returned (done closed).
func sleepOrDone(interval time.Duration, done <-chan struct{}) (stop bool) {
	time.Sleep(interval)
	select {
	case <-done:
		return true
	default:
		return false
	}
}

type IBMCloudClient struct {
	// // The http client for communicating
	http *http.Client

	// Credentials
	IBMApiKey string

	// pollInterval is the wait between resource-status polls. When zero,
	// defaultPollInterval is used. Primarily a seam for tests.
	pollInterval time.Duration
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
	return client.pollUntil(resourceID, resourceType, "ready", timeout, state, client.isResourceReady)
}

// pollUntil repeatedly invokes check until it reports the resource has reached
// its goal state, the timeout elapses, or check returns a fatal (non-transient)
// error. Transient errors (5xx/429 responses or network blips) are retried up to
// maxConsecutiveTransientPollFailures consecutive times so a single flaky API
// response doesn't abort an otherwise-healthy, long-running build; the streak
// resets on any successful poll. goal is used only in log/timeout messages
// (e.g. "ready", "stopped").
func (client IBMCloudClient) pollUntil(
	resourceID string,
	resourceType string,
	goal string,
	timeout time.Duration,
	state multistep.StateBag,
	check func(resourceID string, resourceType string, state multistep.StateBag) (bool, error),
) error {
	ui := state.Get("ui").(packer.Ui)
	done := make(chan struct{})
	defer close(done)
	result := make(chan error, 1)

	interval := client.pollInterval
	if interval <= 0 {
		interval = defaultPollInterval
	}

	go func() {
		attempts := 0
		consecutiveTransientFailures := 0
		for {
			attempts += 1
			if attempts%6 == 0 {
				ui.Say(fmt.Sprintf("Waiting time: %d minutes", attempts/6))
			} else {
				ui.Say(".")
			}

			log.Printf("Checking resource state... (attempt: %d)", attempts)
			reached, err := check(resourceID, resourceType, state)

			if err != nil {
				var transient *transientPollError
				if errors.As(err, &transient) {
					consecutiveTransientFailures++
					if consecutiveTransientFailures > maxConsecutiveTransientPollFailures {
						result <- fmt.Errorf("giving up after %d consecutive transient errors polling %s status: %w",
							consecutiveTransientFailures, resourceType, err)
						return
					}
					ui.Say(fmt.Sprintf("Transient error polling %s status (%d/%d), retrying: %s",
						resourceType, consecutiveTransientFailures, maxConsecutiveTransientPollFailures, err))
					log.Printf("transient error polling %s status (attempt %d, consecutive %d/%d): %s",
						resourceType, attempts, consecutiveTransientFailures, maxConsecutiveTransientPollFailures, err)
				} else {
					result <- err
					return
				}
			} else {
				// A successful poll clears the transient-failure streak.
				consecutiveTransientFailures = 0
				if reached {
					result <- nil
					return
				}
			}

			if sleepOrDone(interval, done) {
				return
			}
		}
	}()

	log.Printf("Waiting for up to %d seconds for resource to become %s", timeout/time.Second, goal)
	select {
	case err := <-result:
		return err
	case <-time.After(timeout):
		err := fmt.Errorf("timeout while waiting for the resource to become %s", goal)
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
		instance, resp, err := vpcService.GetInstance(options)
		if err != nil {
			wrapped := fmt.Errorf("[ERROR] Error occurred while getting instance information. Error: %s", err)
			return false, classifyPollError(resp, err, wrapped)
		}
		status := *instance.Status
		if status == "failed" {
			err := fmt.Errorf("[ERROR] Instance return with failed status. Status Reason - %s: %s", status, *instance.StatusReasons[0].Message)
			return false, err
		}
		ready = status == "running"
		return ready, err
	} else if resourceType == "floating_ips" {
		options := vpcService.NewGetFloatingIPOptions(resourceID)
		floatingIP, resp, err := vpcService.GetFloatingIP(options)
		if err != nil {
			wrapped := fmt.Errorf("[ERROR] Error occurred while getting floating ip information. Error: %s", err)
			return false, classifyPollError(resp, err, wrapped)
		}
		status := *floatingIP.Status
		ready = status == "available"
		return ready, err
	} else if resourceType == "subnets" {
		options := vpcService.NewGetSubnetOptions(resourceID)
		subnet, resp, err := vpcService.GetSubnet(options)
		if err != nil {
			wrapped := fmt.Errorf("[ERROR] Error occurred while getting subnet information. Error: %s", err)
			return false, classifyPollError(resp, err, wrapped)
		}
		status := *subnet.Status
		ready = status == "available"
		return ready, err
	} else if resourceType == "images" {
		options := vpcService.NewGetImageOptions(resourceID)
		image, resp, err := vpcService.GetImage(options)
		if err != nil {
			wrapped := fmt.Errorf("[ERROR] Error occurred while getting image information. Error: %s", err)
			return false, classifyPollError(resp, err, wrapped)
		}
		status := *image.Status
		ready = status == "available"
		if status == "failed" {
			err = fmt.Errorf("[ERROR] Image went into failed state")
		}
		return ready, err
	}
	return ready, nil
}

func (client IBMCloudClient) waitForResourceDown(resourceID string, resourceType string, timeout time.Duration, state multistep.StateBag) error {
	return client.pollUntil(resourceID, resourceType, "stopped", timeout, state, client.isResourceDown)
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
		instance, resp, err := vpcService.GetInstance(options)
		if err != nil {
			// Return the classified error and let pollUntil decide whether to
			// retry (transient) or surface it; don't ui.Error/log here, since a
			// retried transient blip shouldn't print a scary [ERROR] line.
			wrapped := fmt.Errorf("[ERROR] Failed retrieving resource information. Error: %s", err)
			return false, classifyPollError(resp, err, wrapped)
		}
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
		Target: &vpcv1.FloatingIPTargetPrototype{
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
	content, err := os.ReadFile(file)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error reading SSH Public Key. Error: %s", err)
		ui.Error(err.Error())
		log.Println(err.Error())
		return nil, err
	}

	publicKey := string(content)
	state.Put("ssh_public_key", publicKey)

	options := &vpcv1.CreateKeyOptions{}
	if config.SshKeyType != "" && config.SshKeyType == "ed25519" {
		options.SetType("ed25519")
	}
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
