package ibmcloud

import (
	"fmt"

	"github.com/hashicorp/packer/helper/multistep"
	"golang.org/x/crypto/ssh"
	//"code.google.com/p/go.crypto/ssh"
)

func sshCommHost(state multistep.StateBag) (string, error) {
	config := state.Get("config").(Config)
	return config.Comm.SSHHost, nil
}

func sshConfig(state multistep.StateBag) (*ssh.ClientConfig, error) {
	config := state.Get("config").(Config)
	privateKey := state.Get("ssh_private_key").(string)

	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		return nil, fmt.Errorf("Error setting up SSH config: %s", err)
	}

	return &ssh.ClientConfig{
		User: config.Comm.SSHUsername,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}
