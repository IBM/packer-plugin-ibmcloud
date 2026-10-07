package vpc

import (
	"fmt"
	"log"

	registryimage "github.com/hashicorp/packer-plugin-sdk/packer/registry/image"
)

// Artifact represents a Image volume as the result of a Packer build.
type Artifact struct {
	imageName string
	imageId   string
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
	return fmt.Sprintf("Image Name: %s || Image ID: %s", a.imageName, a.imageId)
}

func (a *Artifact) State(name string) interface{} {
	if value, ok := a.StateData[name]; ok {
		return value
	}

	switch name {
	case registryimage.ArtifactStateURI:
		return a.stateHCPPackerRegistryMetadata()
	default:
		return nil
	}
}

// stateHCPPackerRegistryMetadata constructs HCP Packer registry metadata for the built image
func (a *Artifact) stateHCPPackerRegistryMetadata() interface{} {
	// Missing or non-string values yield an empty string rather than a panic,
	// since HCP metadata is best-effort and must never fail a finished build.
	region, _ := a.StateData["region"].(string)
	sourceImageID, _ := a.StateData["source_image_id"].(string)

	image := &registryimage.Image{
		ImageID:        a.imageId,
		ProviderName:   "ibmcloud-vpc",
		ProviderRegion: region,
		SourceImageID:  sourceImageID,
	}
	return []*registryimage.Image{image}
}

// Destroy destroys the VPC image represented by the artifact.
func (a *Artifact) Destroy() error {
	log.Printf("Destroying image: %s", a.String())
	// err := artifact.client.destroyImage(artifact.imageId)
	return nil
}
