package classic

import (
	"bytes"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"golang.org/x/crypto/ssh"
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
		return nil, fmt.Errorf("error setting up SSH config: %s", err)
	}

	return &ssh.ClientConfig{
		User: config.Comm.SSHUsername,
		Auth: []ssh.AuthMethod{
			ssh.PublicKeys(signer),
		},
		HostKeyCallback: pinnedHostKeyCallback(state),
	}, nil
}

const hostKeysStateKey = "ssh_host_keys"

var hostKeysMu sync.Mutex

// pinnedHostKeyCallback pins the host key presented on the first connection to
// each host (trust on first use) and rejects a different key for the same host
// later in the build. The first connection is not authenticated, so this does
// not protect against an attacker who is on the path at first contact.
func pinnedHostKeyCallback(state multistep.StateBag) ssh.HostKeyCallback {
	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		hostKeysMu.Lock()
		defer hostKeysMu.Unlock()

		pins, _ := state.Get(hostKeysStateKey).(map[string][]byte)
		if pins == nil {
			pins = map[string][]byte{}
			state.Put(hostKeysStateKey, pins)
		}

		presented := key.Marshal()
		pinned, seen := pins[hostname]
		if !seen {
			pins[hostname] = presented
			log.Printf("Pinned SSH host key for %s: %s", hostname, ssh.FingerprintSHA256(key))
			return nil
		}
		if !bytes.Equal(pinned, presented) {
			pinnedKey, err := ssh.ParsePublicKey(pinned)
			pinnedFP := "unknown"
			if err == nil {
				pinnedFP = ssh.FingerprintSHA256(pinnedKey)
			}
			return fmt.Errorf("SSH host key mismatch for %s: expected %s, got %s "+
				"(commonly caused by host key regeneration on boot or a man-in-the-middle)",
				hostname, pinnedFP, ssh.FingerprintSHA256(key))
		}
		return nil
	}
}
