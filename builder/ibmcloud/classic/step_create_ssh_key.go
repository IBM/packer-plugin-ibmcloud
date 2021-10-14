package classic

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

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"github.com/hashicorp/packer-plugin-sdk/uuid"
	"golang.org/x/crypto/ssh"
)

type stepCreateSshKey struct {
	keyId          int64
	PrivateKeyFile string
}

func (s *stepCreateSshKey) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*SoftlayerClient)

	ui.Say("Creating SSH Public and Private Key Pair...")
	keysDirectory := "ssh_keys/"
	privatefilepath := keysDirectory + "id_rsa"
	publicfilepath := keysDirectory + "id_rsa.pub"

	// Create a new directory in the current working directory, if it does not exist
	if _, err := os.Stat(keysDirectory); os.IsNotExist(err) {
		err := os.Mkdir(keysDirectory, 0755)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error, cannot create SSH Keys folder: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
	}

	// Creating new RSA Private key
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2014)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error, cannot generate Private SSH Key: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	privDer := x509.MarshalPKCS1PrivateKey(rsaKey)
	privBlk := pem.Block{
		Type:    "RSA PRIVATE KEY",
		Headers: nil,
		Bytes:   privDer,
	}

	// Writing Private key to file
	if privatefilepath != "" {
		ui.Say(fmt.Sprintf("Writing Private SSH Key to a file %s", privatefilepath))
		privatekey := string(pem.EncodeToMemory(&privBlk))
		privateKey := []byte(fmt.Sprintf("%s\n", privatekey))
		err = ioutil.WriteFile(privatefilepath, privateKey, 0600)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to write Private SSH Key to file: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		err = os.Chmod(privatefilepath, 0600)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to edit Private SSH Key's permission: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
	}
	// Set the private key in the statebag for later
	state.Put("ssh_private_key", string(pem.EncodeToMemory(&privBlk)))

	// Creating new RSA Public key
	pub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error, cannot generate SSH Public Key: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return multistep.ActionHalt
	}
	publicKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))

	// Writing Public key to file
	if publicfilepath != "" {
		ui.Say(fmt.Sprintf("Writing Public SSH Key to a file %s", publicfilepath))
		pubkey := string(publicKey)
		pubKey := []byte(fmt.Sprintf("%s\n", pubkey))
		err = ioutil.WriteFile(publicfilepath, pubKey, 0600)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to write Public SSH Key to file: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		err = os.Chmod(publicfilepath, 0600)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to edit Public SSH Key's permission: %s", err)
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

	s.keyId = keyId
	state.Put("ssh_key_id", keyId)
	ui.Say(fmt.Sprintf("Created SSH key with id '%d'", keyId))
	ui.Say("Public and Private SSH Key Pair successfully created.")
	return multistep.ActionContinue
}

func (s *stepCreateSshKey) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	client := state.Get("client").(*SoftlayerClient)

	ui.Say("Deleting Public and Private SSH Key Pair...")
	// If no key name is set, then we never created it, so just return
	if s.keyId == 0 {
		return
	}
	err2 := client.DestroySshKey(s.keyId)
	if err2 != nil {
		log.Printf("Error cleaning up ssh key: %v", err2.Error())
		ui.Error(fmt.Sprintf("Error cleaning up ssh key. Please delete the key (%d) manually", s.keyId))
	}

	ui.Say("Deleting Directory with Public and Private SSH Key Pair...")
	err := os.RemoveAll("ssh_keys")
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed to delete SSH Key folder: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return
	}

	ui.Say("Public and Private SSH Key Pair successfully deleted.")
}
