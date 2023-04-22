package ibmcloudexport

import (
	"fmt"
	"log"

	packersdk "github.com/hashicorp/packer-plugin-sdk/packer"
)

const BuilderId = "ibmcloud.post-processor.vpc-export"

type Artifact struct {
	imageId           string
	imageeExportJobId string

	// StateData should store data such as GeneratedData
	StateData map[string]interface{}
}

var _ packersdk.Artifact = new(Artifact)

func (*Artifact) BuilderId() string {
	return BuilderId
}

func (a *Artifact) Id() string {
	return a.imageeExportJobId
}

func (a *Artifact) Files() []string {
	return nil
}

func (a *Artifact) String() string {
	return fmt.Sprintf("Image ID: %s || Image Export Job ID: %s", a.imageId, a.imageeExportJobId)
}

func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

func (a *Artifact) Destroy() error {
	log.Printf("Destroying artifacts: %s", a.String())
	return nil
}
