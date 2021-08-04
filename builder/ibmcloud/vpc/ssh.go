package vpc

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

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

	file := os.Getenv("PRIVATE_KEY")
	content, err := ioutil.ReadFile(file)
	if err != nil {
		err := fmt.Errorf("[ERROR] Error loading SSH Private Key: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		log.Fatalf(err.Error())
		return nil, err
	}

	privateKey := string(content)
	state.Put("ssh_private_key", privateKey)
	signer, err := ssh.ParsePrivateKey([]byte(privateKey))
	if err != nil {
		err := fmt.Errorf("[ERROR] Error setting up SSH config: %s", err)
		state.Put("error", err)
		ui.Error(err.Error())
		log.Fatalf(err.Error())
		return nil, err
	}

	return &ssh.ClientConfig{
		User: config.Comm.SSHUsername,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}
