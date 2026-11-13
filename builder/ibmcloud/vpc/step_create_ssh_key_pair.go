package vpc

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	mathrand "math/rand"
	"strings"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

type stepCreateSshKeyPair struct{}

func (s *stepCreateSshKeyPair) Run(_ context.Context, state multistep.StateBag) multistep.StepAction {
	ui := state.Get("ui").(packer.Ui)
	config := state.Get("config").(Config)
	ui.Say("Creating SSH Public and Private Key Pair...")

	if config.SshKeyType != "" && config.SshKeyType == "ed25519" {
		// Generate ED25519 keys
		pubKey, privKey, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error, unable to generate ED25519 keypair: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		publicKey, err := ssh.NewPublicKey(pubKey)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error, unable to create SSH public key from ED25519: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}

		pemKey := &pem.Block{
			Type:  "OPENSSH PRIVATE KEY",
			Bytes: marshalED25519PrivateKey(privKey),
		}
		privateKey := pem.EncodeToMemory(pemKey)
		authorizedKey := ssh.MarshalAuthorizedKey(publicKey)

		ui.Say("ED25519 public and private SSH key pair successfully created.")
		state.Put("ssh_private_key_pem", privateKey)
		state.Put("ssh_public_key", authorizedKey)
	} else {
		// Generate RSA keys
		rsaKey, err := rsa.GenerateKey(rand.Reader, 2048)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error, unable to generate RSA keypair: %s", err)
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
		privateKey := pem.EncodeToMemory(&privBlk)

		pub, err := ssh.NewPublicKey(&rsaKey.PublicKey)
		if err != nil {
			err := fmt.Errorf("[ERROR] Error, unable to create SSH public key from RSA: %s", err)
			state.Put("error", err)
			ui.Error(err.Error())
			return multistep.ActionHalt
		}
		publicKey := strings.TrimSpace(string(ssh.MarshalAuthorizedKey(pub)))
		authorizedKey := []byte(fmt.Sprintf("%s\n", publicKey))

		ui.Say("RSA public and private SSH key pair successfully created.")
		state.Put("ssh_private_key_pem", privateKey)
		state.Put("ssh_public_key", authorizedKey)
	}
	return multistep.ActionContinue
}

func (s *stepCreateSshKeyPair) Cleanup(state multistep.StateBag) {
	ui := state.Get("ui").(packer.Ui)
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
