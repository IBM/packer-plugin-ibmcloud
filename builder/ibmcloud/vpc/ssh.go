package vpc

import (
	"fmt"
	"log"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

func sshCommHost(state multistep.StateBag) (string, error) {
	config := state.Get("config").(Config)
	return config.Comm.SSHHost, nil
}

func sshConfig(state multistep.StateBag) (*ssh.ClientConfig, error) {
	config := state.Get("config").(Config)
	ui := state.Get("ui").(packer.Ui)

	privateKeyBytes := state.Get("ssh_private_key_pem").([]byte)
	if privateKeyBytes == nil {
		err := fmt.Errorf("[ERROR] SSH private key not found in state")
		state.Put("error", err)
		ui.Error(err.Error())
		log.Fatalf("error loading ssh private key: %v", err)
		return nil, err
	}

	privateKey := string(privateKeyBytes)
	state.Put("ssh_private_key", privateKey)
	signer, err := ssh.ParsePrivateKey(privateKeyBytes)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error setting up SSH config: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		log.Fatalf("error setting up ssh config: %v", err)
		return nil, err
	}

	return &ssh.ClientConfig{
		User: config.Comm.SSHUsername,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
			ssh.Password(config.Comm.SSHPassword),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}