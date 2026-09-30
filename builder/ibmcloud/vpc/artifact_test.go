package vpc

import (
	"testing"

	registryimage "github.com/hashicorp/packer-plugin-sdk/packer/registry/image"
)

func newTestArtifact(stateData map[string]interface{}) *Artifact {
	return &Artifact{
		imageName: "my-image",
		imageId:   "r006-image-id",
		StateData: stateData,
	}
}

func TestArtifactStateReturnsStateDataValue(t *testing.T) {
	a := newTestArtifact(map[string]interface{}{"region": "us-south"})

	if got := a.State("region"); got != "us-south" {
		t.Fatalf("State(region) = %v, want us-south", got)
	}
}

func TestArtifactStateUnknownKeyReturnsNil(t *testing.T) {
	a := newTestArtifact(map[string]interface{}{"region": "us-south"})

	if got := a.State("does-not-exist"); got != nil {
		t.Fatalf("State(unknown) = %v, want nil", got)
	}
}

func TestArtifactStateRegistryURIReturnsImage(t *testing.T) {
	a := newTestArtifact(map[string]interface{}{
		"region":          "us-south",
		"source_image_id": "r006-base-image",
	})

	images, ok := a.State(registryimage.ArtifactStateURI).([]*registryimage.Image)
	if !ok {
		t.Fatalf("State(ArtifactStateURI) type = %T, want []*registryimage.Image", a.State(registryimage.ArtifactStateURI))
	}
	if len(images) != 1 {
		t.Fatalf("got %d images, want 1", len(images))
	}
	want := registryimage.Image{
		ImageID:        "r006-image-id",
		ProviderName:   "ibmcloud-vpc",
		ProviderRegion: "us-south",
		SourceImageID:  "r006-base-image",
	}
	got := images[0]
	if got.ImageID != want.ImageID || got.ProviderName != want.ProviderName ||
		got.ProviderRegion != want.ProviderRegion || got.SourceImageID != want.SourceImageID {
		t.Fatalf("image = %+v, want %+v", *got, want)
	}
}

func TestArtifactStateRegistryURIPrefersStateDataEntry(t *testing.T) {
	sentinel := "explicit"
	a := newTestArtifact(map[string]interface{}{registryimage.ArtifactStateURI: sentinel})

	if got := a.State(registryimage.ArtifactStateURI); got != sentinel {
		t.Fatalf("State(ArtifactStateURI) = %v, want StateData value %q", got, sentinel)
	}
}

func TestStateHCPPackerRegistryMetadataMissingOrInvalidData(t *testing.T) {
	tests := []struct {
		name       string
		stateData  map[string]interface{}
		wantRegion string
		wantSource string
	}{
		{
			name:      "nil StateData",
			stateData: nil,
		},
		{
			name:       "missing source_image_id",
			stateData:  map[string]interface{}{"region": "us-south"},
			wantRegion: "us-south",
		},
		{
			name:       "missing region",
			stateData:  map[string]interface{}{"source_image_id": "r006-base-image"},
			wantSource: "r006-base-image",
		},
		{
			name: "non-string values",
			stateData: map[string]interface{}{
				"region":          42,
				"source_image_id": nil,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := newTestArtifact(tt.stateData)

			images, ok := a.stateHCPPackerRegistryMetadata().([]*registryimage.Image)
			if !ok || len(images) != 1 {
				t.Fatalf("got %T %v, want one image", a.stateHCPPackerRegistryMetadata(), images)
			}
			img := images[0]
			if img.ImageID != "r006-image-id" || img.ProviderName != "ibmcloud-vpc" {
				t.Errorf("unexpected image identity: %+v", *img)
			}
			if img.ProviderRegion != tt.wantRegion {
				t.Errorf("ProviderRegion = %q, want %q", img.ProviderRegion, tt.wantRegion)
			}
			if img.SourceImageID != tt.wantSource {
				t.Errorf("SourceImageID = %q, want %q", img.SourceImageID, tt.wantSource)
			}
		})
	}
}
