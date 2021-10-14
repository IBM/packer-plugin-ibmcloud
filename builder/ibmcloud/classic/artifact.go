package classic

import (
	"fmt"
	"log"
)

// Artifact represents a Softlayer image as the result of a Packer build.
type Artifact struct {
	imageName      string
	imageId        string
	datacenterName string
	client         *SoftlayerClient
	// StateData should store data such as GeneratedData to be shared with post-processors
	StateData map[string]interface{}
}

// BuilderId returns the builder Id.
func (*Artifact) BuilderId() string {
	return BuilderId
}

// Files returns the files represented by the artifact.
func (*Artifact) Files() []string {
	return nil
}

// Id returns the Softlayer image ID.
func (a *Artifact) Id() string {
	return a.imageId
}

// String returns the string representation of the artifact.
func (a *Artifact) String() string {
	return fmt.Sprintf("%s::%s (%s)", a.datacenterName, a.imageId, a.imageName)
}

func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

// Destroy destroys the Softlayer image represented by the artifact.
func (a *Artifact) Destroy() error {
	log.Printf("Destroying image: %s", a.String())
	err := a.client.destroyImage(a.imageId)
	return err
}
