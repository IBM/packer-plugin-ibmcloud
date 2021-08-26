package vpc

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

type stepCreateSshKeyPair struct{}

func (s *stepCreateSshKeyPair) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	privatefilepath := os.Getenv("PRIVATE_KEY")
	publicfilepath := os.Getenv("PUBLIC_KEY")

	// CREATING NEW RSA PRIVATE AND PUBLIC KEY
	// Creating new RSA Private key
	ui.Say("Creating RSA Private and Public Key Pair...")
	rsaKey, err := rsa.GenerateKey(rand.Reader, 2014)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error, cannot generate RSA Private Key: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
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
		ui.Say(fmt.Sprintf("Writing temp Private Key to a file %s", privatefilepath))
		privatekey := string(pem.EncodeToMemory(&privBlk))
		privateKey := []byte(fmt.Sprintf("%s\n", privatekey))
		err = ioutil.WriteFile(privatefilepath, privateKey, 0600)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to write temp Private Key to file: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
		err = os.Chmod(privatefilepath, 0600)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to edit temp Private Key's permission: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
	}

	// Creating new RSA Public key
	pub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error, cannot generate RSA Public Key: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		// log.Fatalf(err.Error())
		return multistep.ActionHalt
	}
	publicKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))

	// Writing Public key to file
	if publicfilepath != "" {
		ui.Say(fmt.Sprintf("Writing temp Public Key to a file %s", publicfilepath))
		pubkey := string(publicKey)
		pubKey := []byte(fmt.Sprintf("%s\n", pubkey))
		err = ioutil.WriteFile(publicfilepath, pubKey, 0600)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to write temp Public Key to file: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
		err = os.Chmod(publicfilepath, 0600)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to edit temp Public Key's permission: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			// log.Fatalf(err.Error())
			return multistep.ActionHalt
		}
	}
	ui.Say("RSA Private and Public Key Pair successfully created")
	return multistep.ActionContinue
}

func (s *stepCreateSshKeyPair) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	ui.Say("")
	ui.Say("********************************************************************")
	ui.Say("* Thank you for using IBM Cloud Packer Plugin - VPC Infrastructure *")
	ui.Say("********************************************************************")
}
