package vpc

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	"github.com/hashicorp/packer-plugin-sdk/packer"
	"golang.org/x/crypto/ssh"
)

// StepCleanupTempKeys deletes the authorized_keys line ending in
// " <temporary_key_pair_name>", so Prepare must always set the name when
// ssh_clear_authorized_keys is on, otherwise the step is a silent no-op.
func TestPrepareSetsTemporaryKeyPairNameWhenClearingKeys(t *testing.T) {
	c := validVPCConfig()
	c.Comm.SSHUsername = "root"
	c.Comm.SSHClearAuthorizedKeys = true

	if _, err := c.Prepare(); err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if !strings.HasPrefix(c.Comm.SSHTemporaryKeyPairName, "packer_") {
		t.Errorf("SSHTemporaryKeyPairName = %q, want a generated packer_ name", c.Comm.SSHTemporaryKeyPairName)
	}
}

func TestPrepareKeepsUserTemporaryKeyPairName(t *testing.T) {
	c := validVPCConfig()
	c.Comm.SSHUsername = "root"
	c.Comm.SSHClearAuthorizedKeys = true
	c.Comm.SSHTemporaryKeyPairName = "my-build-key"

	if _, err := c.Prepare(); err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if c.Comm.SSHTemporaryKeyPairName != "my-build-key" {
		t.Errorf("SSHTemporaryKeyPairName = %q, want user value kept", c.Comm.SSHTemporaryKeyPairName)
	}
}

func TestPrepareLeavesNameEmptyWhenNotClearingKeys(t *testing.T) {
	c := validVPCConfig()
	c.Comm.SSHUsername = "root"

	if _, err := c.Prepare(); err != nil {
		t.Fatalf("Prepare returned error: %v", err)
	}
	if c.Comm.SSHTemporaryKeyPairName != "" {
		t.Errorf("SSHTemporaryKeyPairName = %q, want empty (no behaviour change when option is off)", c.Comm.SSHTemporaryKeyPairName)
	}
}

// The generated public key must end with the temporary name so the SDK's sed
// pattern ("/ <name>$/d") matches it, for both key types.
func TestGeneratedPublicKeyCarriesTemporaryName(t *testing.T) {
	for _, keyType := range []string{"", "ed25519"} {
		t.Run("key type "+keyType, func(t *testing.T) {
			dir := t.TempDir()
			t.Chdir(dir)

			c := validVPCConfig()
			c.SshKeyType = keyType
			c.Comm.SSHTemporaryKeyPairName = "packer_test_name"

			state := new(multistep.BasicStateBag)
			state.Put("config", *c)
			state.Put("ui", packer.TestUi(t))

			if action := new(stepCreateSshKeyPair).Run(context.Background(), state); action != multistep.ActionContinue {
				t.Fatalf("Run action = %v, want continue (err=%v)", action, state.Get("error"))
			}

			pubPath := state.Get("PUBLIC_KEY").(string)
			content, err := os.ReadFile(pubPath)
			if err != nil {
				t.Fatal(err)
			}
			line := strings.TrimSpace(string(content))
			if !strings.HasSuffix(line, " packer_test_name") {
				t.Errorf("public key line %q does not end with the temporary name", line)
			}
			// Still a valid authorized_keys entry for the VPC API.
			_, comment, _, _, err := ssh.ParseAuthorizedKey(content)
			if err != nil {
				t.Fatalf("public key no longer parses: %v", err)
			}
			if comment != "packer_test_name" {
				t.Errorf("comment = %q, want packer_test_name", comment)
			}
		})
	}
}

func TestGeneratedPublicKeyUnchangedWithoutName(t *testing.T) {
	t.Chdir(t.TempDir())

	c := validVPCConfig()
	state := new(multistep.BasicStateBag)
	state.Put("config", *c)
	state.Put("ui", packer.TestUi(t))

	if action := new(stepCreateSshKeyPair).Run(context.Background(), state); action != multistep.ActionContinue {
		t.Fatalf("Run action = %v, want continue", action)
	}
	content, err := os.ReadFile(state.Get("PUBLIC_KEY").(string))
	if err != nil {
		t.Fatal(err)
	}
	_, comment, _, _, err := ssh.ParseAuthorizedKey(content)
	if err != nil {
		t.Fatal(err)
	}
	if comment != "" {
		t.Errorf("comment = %q, want none when ssh_clear_authorized_keys is off", comment)
	}
}

// The step must be a no-op for winrm and when the option is off, so adding it
// to the SSH step list cannot change existing builds.
func TestStepCleanupTempKeysNoopWhenDisabled(t *testing.T) {
	c := validVPCConfig()
	c.Comm.SSHClearAuthorizedKeys = false

	step := &commonsteps.StepCleanupTempKeys{Comm: &c.Comm}
	// No "communicator" in state: the step would panic if it tried to run.
	if action := step.Run(context.Background(), new(multistep.BasicStateBag)); action != multistep.ActionContinue {
		t.Errorf("action = %v, want continue", action)
	}
}
