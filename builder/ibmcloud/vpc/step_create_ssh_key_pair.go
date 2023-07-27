package vpc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"io/ioutil"
	mathrand "math/rand"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

type stepCreateSshKeyPair struct{}

func (s *stepCreateSshKeyPair) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	nsec := time.Now().UnixNano()
	config := state.Get("config").(Config)
	ui.Say("Creating SSH Public and Private Key Pair...")
	keysDirectory := strconv.FormatInt(nsec, 10) + "ssh_keys/"
	state.Put("keysDirectory", keysDirectory)

	privatefilepath := ""
	publicfilepath := ""
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
	if config.SshKeyType != "" && config.SshKeyType == "ed25519" {
		// for ed25519
		// generate Keys
		pubKey, privKey, _ := ed25519.GenerateKey(rand.Reader)
		publicKey, _ := ssh.NewPublicKey(pubKey)

		pemKey := &pem.Block{
			Type:  "OPENSSH PRIVATE KEY",
			Bytes: marshalED25519PrivateKey(privKey), // converts a ed25519 private key to openssh format
		}
		privateKey := pem.EncodeToMemory(pemKey)
		authorizedKey := ssh.MarshalAuthorizedKey(publicKey)
		privatefilepath = keysDirectory + "id_ed25519"
		publicfilepath = keysDirectory + "id_ed25519.pub"

		err := ioutil.WriteFile(privatefilepath, privateKey, 0600)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to edit ed25519 private SSH Key's permission: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		err = ioutil.WriteFile(publicfilepath, authorizedKey, 0644)
		if err != nil {
			err := fmt.Errorf("[ERROR] Failed to write ed25519 public SSH Key to file: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		ui.Say("ed25519 public and private SSH key pair successfully created.")
		state.Put("PRIVATE_KEY", privatefilepath)
		state.Put("PUBLIC_KEY", publicfilepath)
	} else {
		// Create a new directory in the current working directory, if it does not exist
		privatefilepath = keysDirectory + "id_rsa"
		publicfilepath = keysDirectory + "id_rsa.pub"

		// Creating new RSA Private key
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2014)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error, unable to generate ED25519 keypair: %s", err)
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

		// Creating new RSA Public key
		pub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error, unable to generate RSA keypair: %s", err)
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

		ui.Say("RSA public and private SSH key pair successfully created.")
		state.Put("PRIVATE_KEY", privatefilepath)
		state.Put("PUBLIC_KEY", publicfilepath)
	}
	return multistep.ActionContinue
}

func (s *stepCreateSshKeyPair) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
	keysDirectory := state.Get("keysDirectory").(string)
	ui.Say("Deleting Public and Private SSH Key Pair...")
	err := os.RemoveAll(keysDirectory)
	if err != nil {
		err := fmt.Errorf("[ERROR] Failed to delete SSH Key folder: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		return
	}
	ui.Say("Public and Private SSH Key Pair successfully deleted.")

	ui.Say("")
	ui.Say("********************************************************************")
	ui.Say("* Thank you for using IBM Cloud Packer Plugin - VPC Infrastructure *")
	ui.Say("********************************************************************")
}

func marshalED25519PrivateKey(key ed25519.PrivateKey) []byte {
	// Add key header (followed by a null byte)
	marshal := append([]byte("openssh-key-v1"), 0)

	var w struct {
		CipherName   string
		KdfName      string
		KdfOpts      string
		NumKeys      uint32
		PubKey       []byte
		PrivKeyBlock []byte
	}

	// Fill out the private key fields
	pk1 := struct {
		Check1  uint32
		Check2  uint32
		Keytype string
		Pub     []byte
		Priv    []byte
		Comment string
		Pad     []byte `ssh:"rest"`
	}{}

	// Set check ints
	ci := mathrand.Uint32()
	pk1.Check1 = ci
	pk1.Check2 = ci

	// Set key type
	pk1.Keytype = ssh.KeyAlgoED25519

	// Add the pubkey to the optionally-encrypted block
	pk, ok := key.Public().(ed25519.PublicKey)
	if !ok {
		return nil
	}
	pubKey := []byte(pk)
	pk1.Pub = pubKey

	// Add private key
	pk1.Priv = []byte(key)

	// Add null comment
	pk1.Comment = ""

	// Add padding to match the encryption block size within PrivKeyBlock (without Pad field)
	// 8 doesn't match the documentation, ssh-keygen uses that for unencrypted keys.
	bs := 8
	blockLen := len(ssh.Marshal(pk1))
	padLen := (bs - (blockLen % bs)) % bs
	pk1.Pad = make([]byte, padLen)

	// Padding is a sequence of bytes like: 1, 2, 3...
	for i := 0; i < padLen; i++ {
		pk1.Pad[i] = byte(i + 1)
	}

	// Generate the pubkey prefix "\0\0\0\nssh-ed25519\0\0\0 "
	prefix := []byte{0x0, 0x0, 0x0, 0x0b}
	prefix = append(prefix, []byte(ssh.KeyAlgoED25519)...)
	prefix = append(prefix, []byte{0x0, 0x0, 0x0, 0x20}...)

	// support unencrypted keys for now
	w.CipherName = "none"
	w.KdfName = "none"
	w.KdfOpts = ""
	w.NumKeys = 1
	w.PubKey = append(prefix, pubKey...)
	w.PrivKeyBlock = ssh.Marshal(pk1)

	marshal = append(marshal, ssh.Marshal(w)...)

	return marshal
}
