package classic

import (
	"context"
	"log"

	"github.com/hashicorp/hcl/v2/hcldec"
	"github.com/hashicorp/packer-plugin-sdk/communicator"
	"github.com/hashicorp/packer-plugin-sdk/multistep"
	"github.com/hashicorp/packer-plugin-sdk/multistep/commonsteps"
	"github.com/hashicorp/packer-plugin-sdk/packer"
)

// The unique ID for this builder.
const BuilderId = "ibmcloud.classic.builder"

// Image Types
//const IMAGE_TYPE_FLEX = "flex" //----NOT SUPPORTED
const IMAGE_TYPE_STANDARD = "standard"

// Builder represents a Packer Builder.
type Builder struct {
	config Config
	runner multistep.Runner
}

func (b *Builder) ConfigSpec() hcldec.ObjectSpec {
	return b.config.FlatMapstructure().HCL2Spec()
}

func (b *Builder) Prepare(raws ...interface{}) ([]string, []string, error) {
	warnings, errs := b.config.Prepare(raws...)
	if errs != nil {
		return nil, warnings, errs
	}

	// Return the placeholder for the generated data that will become available to provisioners and post-processors.
	// If the builder doesn't generate any data, just return an empty slice of string: []string{}
	buildGeneratedData := []string{}
	return buildGeneratedData, nil, nil
}

// Run executes a SoftLayer Packer build and returns a packer.Artifact
// representing a SoftLayer machine image (standard).
// func (self *Builder) Run(ui packer.Ui, hook packer.Hook, cache packer.Cache) (packer.Artifact, error) {
func (b *Builder) Run(ctx context.Context, ui packer.Ui, hook packer.Hook) (packer.Artifact, error) {

	// Create the client
	client := SoftlayerClient{}.New(b.config.Username, b.config.APIKey)

	// Set up the state which is used to share state between the steps
	state := new(multistep.BasicStateBag)
	state.Put("config", b.config)
	state.Put("client", client)
	state.Put("hook", hook)
	state.Put("ui", ui)

	// Set the value of the generated data that will become available to provisioners.
	// To share the data with post-processors, use the StateData in the artifact.
	state.Put("generated_data", map[string]interface{}{
		"GeneratedMockData": "mock-build-data",
	})

	// Build the steps
	steps := []multistep.Step{}
	if b.config.Comm.Type == "winrm" {
		steps = []multistep.Step{
			new(stepCreateInstance),
			new(stepWaitforInstance),
			new(stepGrabPublicIP),
			&communicator.StepConnect{
				Config:      &b.config.Comm,
				Host:        winRMCommHost,
				WinRMConfig: winRMConfig,
			},
			new(stepWaitforInstance),
			new(commonsteps.StepProvision),
			new(stepCaptureImage),
		}
	} else if b.config.Comm.Type == "ssh" {
		steps = []multistep.Step{
			&stepCreateSshKey{
				PrivateKeyFile: string(b.config.Comm.SSHPrivateKey),
			},
			new(stepCreateInstance),
			new(stepWaitforInstance),
			new(stepGrabPublicIP),
			&communicator.StepConnect{
				Config:    &b.config.Comm,
				Host:      sshCommHost,
				SSHConfig: sshConfig,
			},
			new(commonsteps.StepProvision),
			new(stepCaptureImage),
		}
	}

	// Create the runner which will run the steps we just build
	b.runner = &multistep.BasicRunner{Steps: steps}
	b.runner.Run(ctx, state)

	// If there was an error, return that
	if rawErr, ok := state.GetOk("error"); ok {
		return nil, rawErr.(error)
	}

	if _, ok := state.GetOk("image_id"); !ok {
		log.Println("Failed to find image_id in state. Bug?")
		return nil, nil
	}

	// Create an artifact and return it
	artifact := &Artifact{
		imageName:      b.config.ImageName,
		imageId:        state.Get("image_id").(string),
		datacenterName: b.config.DatacenterName,
		client:         client,

		// Add the builder generated data to the artifact StateData so that post-processors can access them.
		StateData: map[string]interface{}{"generated_data": state.Get("generated_data")},
	}

	return artifact, nil
}
