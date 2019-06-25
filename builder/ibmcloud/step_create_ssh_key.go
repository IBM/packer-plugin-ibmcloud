package ibmcloud

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/hashicorp/packer/common/uuid"
	"github.com/hashicorp/packer/helper/multistep"
	"github.com/hashicorp/packer/packer"
	"golang.org/x/crypto/ssh"
)

type stepCreateSshKey struct {
	keyId          int64
	PrivateKeyFile string
}

func (self *stepCreateSshKey) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*SoftlayerClient)
	ui.Say("Creating temporary ssh key for the instance...")

	rsaKey, err := rsa.GenerateKey(rand.Reader, 2014)
	if err != nil {
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	// ASN.1 DER encoded form
	privDer := x509.MarshalPKCS1PrivateKey(rsaKey)
	privBlk := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDer,
	}

	privatefilepath := os.Getenv("PRIVATEKEY")
	if privatefilepath != "" {
		ui.Say(fmt.Sprintf("Writing private key to a file %s", privatefilepath))
		privatekey := string(pem.EncodeToMemory(&privBlk))
		privateKey := []byte(fmt.Sprintf("%s\n", privatekey))
		err = ioutil.WriteFile(privatefilepath, privateKey, 0600)
		if err != nil {
			err := fmt.Errorf("Failed to write private key to file %s", privatefilepath)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		err = os.Chmod(privatefilepath, 0600)
		if err != nil {
			err := fmt.Errorf("Failed to edit private file permission: %s", privatefilepath)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}
	// Set the private key in the statebag for later
	state.Put("ssh_private_key", string(pem.EncodeToMemory(&privBlk)))

	pub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	if err != nil {
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}
	publicKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
	publicfilepath := os.Getenv("PUBLICKEY")
	if publicfilepath != "" {
		ui.Say(fmt.Sprintf("Writing public key to a file %s", publicfilepath))
		pubkey := string(publicKey)
		pubKey := []byte(fmt.Sprintf("%s\n", pubkey))
		err = ioutil.WriteFile(publicfilepath, pubKey, 0600)
		if err != nil {
			err := fmt.Errorf("Failed to edit private file permission: %s", publicfilepath)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		err = os.Chmod(publicfilepath, 0600)
		if err != nil {
			err := fmt.Errorf("Failed to write public key to file %s", publicfilepath)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}
	// The name of the public key
	label := fmt.Sprintf("packer-%s", uuid.TimeOrderedUUID())
	keyId, err := client.UploadSshKey(label, publicKey)
	if err != nil {
		ui.Error(err.Error())
		state.Put("error", err)
		return multistep.ActionHalt
	}

	self.keyId = keyId
	state.Put("ssh_key_id", keyId)

	ui.Say(fmt.Sprintf("Created SSH key with id '%d'", keyId))

	return multistep.ActionContinue
}

func (self *stepCreateSshKey) Cleanup(state multistep.StateBag) {
	// If no key name is set, then we never created it, so just return
	if self.keyId == 0 {
		return
	}

	client := state.Get("client").(*SoftlayerClient)
	ui := state.Get("ui").(packer.Ui)
	log.Printf("Cleaning up ssh key")
	ui.Say("Deleting temporary ssh key...")
	err := client.DestroySshKey(self.keyId)

	if err != nil {
		log.Printf("Error cleaning up ssh key: %v", err.Error())
		ui.Error(fmt.Sprintf("Error cleaning up ssh key. Please delete the key (%d) manually", self.keyId))
	}
}
