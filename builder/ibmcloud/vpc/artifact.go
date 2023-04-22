package vpc

import (
	"fmt"
	"log"
)

// Artifact represents a Image volume as the result of a Packer build.
type Artifact struct {
	imageName string
	imageId   string
	ibmApiKey string
	client    *IBMCloudClient

	// StateData should store data such as GeneratedData to be shared with post-processors
	StateData map[string]interface{}
}

// BuilderId returns the builder Id.
func (*Artifact) BuilderId() string {
	return BuilderId
}

// Files returns the files represented by the artifact.
func (a *Artifact) Files() []string {
	return nil
}

// Id returns the IBMCloud image ID.
func (a *Artifact) Id() string {
	return a.imageId
}

// String returns the string representation of the artifact.
func (a *Artifact) String() string {
	return fmt.Sprintf("Image Name: %s || Image ID: %s || IBM API Key ID: %s ", a.imageName, a.imageId, a.ibmApiKey)
}

func (a *Artifact) State(name string) interface{} {
	return a.StateData[name]
}

// Destroy destroys the VPC image represented by the artifact.
func (a *Artifact) Destroy() error {
	log.Printf("Destroying image: %s", a.String())
	// err := artifact.client.destroyImage(artifact.imageId)
	return nil
}
