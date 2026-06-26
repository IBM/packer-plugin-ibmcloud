package vpc

import (
	"strings"
	"testing"

	"github.com/IBM/vpc-go-sdk/vpcv1"
)

// otherwise-valid config so Prepare surfaces only the boot-volume checks.
func validVPCConfig() *Config {
	c := &Config{}
	c.Comm.Type = "ssh"
	c.IBMApiKey = "test-api-key"
	c.Region = "us-east"
	c.SubnetID = "0717-test-subnet"
	c.VSIBaseImageID = "r014-test-image"
	c.VSIProfile = "bz2-4x16"
	c.VSIBootCapacity = 100
	return c
}

func TestPrepareBootVolumeCapacity(t *testing.T) {
	const wantMsg = "boot capacity out of bound"

	cases := []struct {
		name       string
		capacity   int
		wantReject bool
	}{
		{"zero uses image default", 0, false},
		{"minimum 10", 10, false},
		{"just below minimum", 9, true},
		{"mid range", 50, false},
		{"sdp range above old limit", 1000, false},
		{"maximum 32000", 32000, false},
		{"just above maximum", 32001, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			c.VSIBootCapacity = tc.capacity

			_, err := c.Prepare()

			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("VSIBootCapacity=%d: boot-capacity rejected=%v, want %v (err=%v)",
					tc.capacity, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestPrepareBootVolumeProfile(t *testing.T) {
	const wantMsg = "vsi_boot_vol_profile must be one of"

	cases := []struct {
		name       string
		profile    string
		wantReject bool
	}{
		{"empty uses default", "", false},
		{"general-purpose", "general-purpose", false},
		{"5iops-tier", "5iops-tier", false},
		{"10iops-tier", "10iops-tier", false},
		{"sdp", "sdp", false},
		{"custom", "custom", false},
		{"unknown profile", "platinum-tier", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			c.VSIBootProfile = tc.profile
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("profile=%q rejected=%v, want %v (err=%v)", tc.profile, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestPrepareBootVolumeIopsBandwidth(t *testing.T) {
	const wantMsg = "require vsi_boot_vol_profile to be 'custom' or 'sdp'"

	cases := []struct {
		name       string
		profile    string
		iops       int
		bandwidth  int
		wantReject bool
	}{
		{"iops with sdp", "sdp", 10000, 0, false},
		{"bandwidth with sdp", "sdp", 0, 4000, false},
		{"iops with custom", "custom", 5000, 0, false},
		{"none set", "general-purpose", 0, 0, false},
		{"iops without a profile", "", 5000, 0, true},
		{"iops with a tiered profile", "general-purpose", 5000, 0, true},
		{"bandwidth with a tiered profile", "10iops-tier", 0, 2000, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			c.VSIBootProfile = tc.profile
			c.VSIBootIops = tc.iops
			c.VSIBootBandwidth = tc.bandwidth
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("profile=%q iops=%d bandwidth=%d rejected=%v, want %v (err=%v)",
					tc.profile, tc.iops, tc.bandwidth, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestPrepareBootVolumeRequiresCapacity(t *testing.T) {
	const wantMsg = "require vsi_boot_vol_capacity to be set"

	cases := []struct {
		name       string
		profile    string
		iops       int
		bandwidth  int
		capacity   int
		snapshot   bool // use the create-from-snapshot path instead of by-image
		wantReject bool
	}{
		{"profile without capacity", "sdp", 0, 0, 0, false, true},
		{"iops without capacity", "sdp", 10000, 0, 0, false, true},
		{"bandwidth without capacity", "sdp", 0, 4000, 0, false, true},
		{"profile with capacity", "sdp", 0, 0, 100, false, false},
		{"nothing set, no capacity", "", 0, 0, 0, false, false},
		// Snapshot path: capacity is optional (the restored volume inherits the
		// snapshot's size), so a profile/iops/bandwidth without capacity is allowed.
		{"snapshot profile without capacity", "sdp", 0, 0, 0, true, false},
		{"snapshot iops without capacity", "sdp", 10000, 0, 0, true, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			if tc.snapshot {
				c.VSIBaseImageID = ""
				c.VSIBootSnapshotID = "r006-test-snapshot"
			}
			c.VSIBootCapacity = tc.capacity
			c.VSIBootProfile = tc.profile
			c.VSIBootIops = tc.iops
			c.VSIBootBandwidth = tc.bandwidth
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("profile=%q iops=%d bandwidth=%d capacity=%d snapshot=%v rejected=%v, want %v (err=%v)",
					tc.profile, tc.iops, tc.bandwidth, tc.capacity, tc.snapshot, rejected, tc.wantReject, err)
			}
		})
	}
}

// Attaching an existing volume by ID must reject any boot-volume tuning fields,
// since the by-ID create path ignores them — accepting them would silently drop
// the user's settings.
func TestPrepareBootVolumeByIDRejectsTuning(t *testing.T) {
	const wantMsg = "cannot be combined with vsi_boot_volume_id"

	cases := []struct {
		name       string
		profile    string
		iops       int
		bandwidth  int
		capacity   int
		wantReject bool
	}{
		{"profile", "sdp", 0, 0, 0, true},
		{"iops", "", 10000, 0, 0, true},
		{"bandwidth", "", 0, 4000, 0, true},
		{"capacity", "", 0, 0, 100, true},
		{"nothing extra", "", 0, 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			// Switch to the by-ID path (mutually exclusive with the base image).
			c.VSIBaseImageID = ""
			c.VSIBootVolumeID = "r006-test-volume"
			c.VSIBootCapacity = tc.capacity
			c.VSIBootProfile = tc.profile
			c.VSIBootIops = tc.iops
			c.VSIBootBandwidth = tc.bandwidth
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("profile=%q iops=%d bandwidth=%d capacity=%d rejected=%v, want %v (err=%v)",
					tc.profile, tc.iops, tc.bandwidth, tc.capacity, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestPrepareBootVolumeNegative(t *testing.T) {
	const wantMsg = "must not be negative"

	cases := []struct {
		name       string
		iops       int
		bandwidth  int
		wantReject bool
	}{
		{"negative iops", -1, 0, true},
		{"negative bandwidth", 0, -1, true},
		{"positive values", 10000, 4000, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			c.VSIBootProfile = "sdp"
			c.VSIBootIops = tc.iops
			c.VSIBootBandwidth = tc.bandwidth
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("iops=%d bandwidth=%d rejected=%v, want %v (err=%v)",
					tc.iops, tc.bandwidth, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestBootVolumePrototype(t *testing.T) {
	t.Run("default profile, no iops or bandwidth", func(t *testing.T) {
		vol := bootVolumePrototype(&Config{VSIBootCapacity: 100})
		if got := *vol.Profile.(*vpcv1.VolumeProfileIdentity).Name; got != "general-purpose" {
			t.Errorf("profile = %q, want general-purpose", got)
		}
		if got := *vol.Capacity; got != 100 {
			t.Errorf("capacity = %d, want 100", got)
		}
		if vol.Iops != nil {
			t.Errorf("Iops = %d, want nil (unset)", *vol.Iops)
		}
		if vol.Bandwidth != nil {
			t.Errorf("Bandwidth = %d, want nil (unset)", *vol.Bandwidth)
		}
	})

	// iops and bandwidth are set from two independent blocks, so verify each is
	// honored on its own (guards against a copy-paste bug coupling the two).
	t.Run("custom profile with iops only", func(t *testing.T) {
		vol := bootVolumePrototype(&Config{
			VSIBootCapacity: 100,
			VSIBootProfile:  "custom",
			VSIBootIops:     5000,
		})
		if got := *vol.Profile.(*vpcv1.VolumeProfileIdentity).Name; got != "custom" {
			t.Errorf("profile = %q, want custom", got)
		}
		if vol.Iops == nil || *vol.Iops != 5000 {
			t.Errorf("Iops = %v, want 5000", vol.Iops)
		}
		if vol.Bandwidth != nil {
			t.Errorf("Bandwidth = %d, want nil (unset)", *vol.Bandwidth)
		}
	})

	t.Run("sdp profile with bandwidth only", func(t *testing.T) {
		vol := bootVolumePrototype(&Config{
			VSIBootCapacity:  100,
			VSIBootProfile:   "sdp",
			VSIBootBandwidth: 4000,
		})
		if vol.Bandwidth == nil || *vol.Bandwidth != 4000 {
			t.Errorf("Bandwidth = %v, want 4000", vol.Bandwidth)
		}
		if vol.Iops != nil {
			t.Errorf("Iops = %d, want nil (unset)", *vol.Iops)
		}
	})

	t.Run("sdp profile with iops and bandwidth", func(t *testing.T) {
		vol := bootVolumePrototype(&Config{
			VSIBootCapacity:  50,
			VSIBootProfile:   "sdp",
			VSIBootIops:      10000,
			VSIBootBandwidth: 4000,
		})
		if got := *vol.Profile.(*vpcv1.VolumeProfileIdentity).Name; got != "sdp" {
			t.Errorf("profile = %q, want sdp", got)
		}
		if vol.Iops == nil || *vol.Iops != 10000 {
			t.Errorf("Iops = %v, want 10000", vol.Iops)
		}
		if vol.Bandwidth == nil || *vol.Bandwidth != 4000 {
			t.Errorf("Bandwidth = %v, want 4000", vol.Bandwidth)
		}
	})
}

func TestSnapshotBootVolumePrototype(t *testing.T) {
	snap := &vpcv1.SnapshotIdentity{ID: &[]string{"r006-snap"}[0]}

	t.Run("defaults: profile general-purpose, capacity inherited from snapshot", func(t *testing.T) {
		vol := snapshotBootVolumePrototype(&Config{}, snap)
		if got := *vol.Profile.(*vpcv1.VolumeProfileIdentity).Name; got != "general-purpose" {
			t.Errorf("profile = %q, want general-purpose", got)
		}
		// Capacity unset means the restored volume inherits the snapshot's size.
		if vol.Capacity != nil {
			t.Errorf("Capacity = %d, want nil (inherit from snapshot)", *vol.Capacity)
		}
		if vol.Iops != nil {
			t.Errorf("Iops = %d, want nil (unset)", *vol.Iops)
		}
		if vol.Bandwidth != nil {
			t.Errorf("Bandwidth = %d, want nil (unset)", *vol.Bandwidth)
		}
	})

	t.Run("sdp profile with capacity, iops and bandwidth", func(t *testing.T) {
		vol := snapshotBootVolumePrototype(&Config{
			VSIBootCapacity:  120,
			VSIBootProfile:   "sdp",
			VSIBootIops:      10000,
			VSIBootBandwidth: 4000,
		}, snap)
		if got := *vol.Profile.(*vpcv1.VolumeProfileIdentity).Name; got != "sdp" {
			t.Errorf("profile = %q, want sdp", got)
		}
		if vol.Capacity == nil || *vol.Capacity != 120 {
			t.Errorf("Capacity = %v, want 120", vol.Capacity)
		}
		if vol.Iops == nil || *vol.Iops != 10000 {
			t.Errorf("Iops = %v, want 10000", vol.Iops)
		}
		if vol.Bandwidth == nil || *vol.Bandwidth != 4000 {
			t.Errorf("Bandwidth = %v, want 4000", vol.Bandwidth)
		}
		if vol.SourceSnapshot != snap {
			t.Error("SourceSnapshot was not propagated")
		}
	})
}

func TestPrepareDataVolumeCapacity(t *testing.T) {
	const wantMsg = "data capacity out of bound"

	cases := []struct {
		name       string
		capacity   int
		wantReject bool
	}{
		{"zero means no data volume", 0, false},
		{"minimum 10", 10, false},
		{"just below minimum", 9, true},
		{"mid range", 60, false},
		{"maximum 32000", 32000, false},
		{"just above maximum", 32001, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			c.VSIDataCapacity = tc.capacity
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("VSIDataCapacity=%d rejected=%v, want %v (err=%v)",
					tc.capacity, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestPrepareDataVolumeProfile(t *testing.T) {
	const wantMsg = "vsi_data_vol_profile must be one of"

	cases := []struct {
		name       string
		profile    string
		wantReject bool
	}{
		{"empty uses default", "", false},
		{"general-purpose", "general-purpose", false},
		{"5iops-tier", "5iops-tier", false},
		{"10iops-tier", "10iops-tier", false},
		{"sdp", "sdp", false},
		{"custom", "custom", false},
		{"unknown profile", "platinum-tier", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			c.VSIDataCapacity = 60
			c.VSIDataProfile = tc.profile
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("profile=%q rejected=%v, want %v (err=%v)", tc.profile, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestPrepareDataVolumeIopsBandwidth(t *testing.T) {
	const wantMsg = "require vsi_data_vol_profile to be 'custom' or 'sdp'"

	cases := []struct {
		name       string
		profile    string
		iops       int
		bandwidth  int
		wantReject bool
	}{
		{"iops with sdp", "sdp", 10000, 0, false},
		{"bandwidth with sdp", "sdp", 0, 2000, false},
		{"iops with custom", "custom", 5000, 0, false},
		{"none set", "general-purpose", 0, 0, false},
		{"iops without a profile", "", 5000, 0, true},
		{"iops with a tiered profile", "general-purpose", 5000, 0, true},
		{"bandwidth with a tiered profile", "10iops-tier", 0, 2000, true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			c.VSIDataCapacity = 60
			c.VSIDataProfile = tc.profile
			c.VSIDataIops = tc.iops
			c.VSIDataBandwidth = tc.bandwidth
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("profile=%q iops=%d bandwidth=%d rejected=%v, want %v (err=%v)",
					tc.profile, tc.iops, tc.bandwidth, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestPrepareDataVolumeRequiresCapacity(t *testing.T) {
	const wantMsg = "require vsi_data_vol_capacity to be set"

	cases := []struct {
		name       string
		profile    string
		iops       int
		bandwidth  int
		capacity   int
		wantReject bool
	}{
		{"profile without capacity", "sdp", 0, 0, 0, true},
		{"iops without capacity", "sdp", 10000, 0, 0, true},
		{"bandwidth without capacity", "sdp", 0, 2000, 0, true},
		{"profile with capacity", "sdp", 0, 0, 60, false},
		{"nothing set, no capacity", "", 0, 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			c.VSIDataCapacity = tc.capacity
			c.VSIDataProfile = tc.profile
			c.VSIDataIops = tc.iops
			c.VSIDataBandwidth = tc.bandwidth
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("profile=%q iops=%d bandwidth=%d capacity=%d rejected=%v, want %v (err=%v)",
					tc.profile, tc.iops, tc.bandwidth, tc.capacity, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestPrepareDataVolumeNegative(t *testing.T) {
	const wantMsg = "vsi_data_vol_iops and vsi_data_vol_bandwidth must not be negative"

	cases := []struct {
		name       string
		iops       int
		bandwidth  int
		wantReject bool
	}{
		{"negative iops", -1, 0, true},
		{"negative bandwidth", 0, -1, true},
		{"positive values", 10000, 2000, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
			c.VSIDataCapacity = 60
			c.VSIDataProfile = "sdp"
			c.VSIDataIops = tc.iops
			c.VSIDataBandwidth = tc.bandwidth
			_, err := c.Prepare()
			rejected := err != nil && strings.Contains(err.Error(), wantMsg)
			if rejected != tc.wantReject {
				t.Errorf("iops=%d bandwidth=%d rejected=%v, want %v (err=%v)",
					tc.iops, tc.bandwidth, rejected, tc.wantReject, err)
			}
		})
	}
}

func TestDataVolumeAttachments(t *testing.T) {
	t.Run("no capacity attaches nothing", func(t *testing.T) {
		if att := dataVolumeAttachments(&Config{}); att != nil {
			t.Errorf("attachments = %v, want nil when no data volume is configured", att)
		}
	})

	t.Run("default profile, deleted with instance, no iops or bandwidth", func(t *testing.T) {
		att := dataVolumeAttachments(&Config{VSIDataCapacity: 60})
		if len(att) != 1 {
			t.Fatalf("len(attachments) = %d, want 1", len(att))
		}
		if att[0].DeleteVolumeOnInstanceDelete == nil || !*att[0].DeleteVolumeOnInstanceDelete {
			t.Error("DeleteVolumeOnInstanceDelete should be true so the scratch volume is torn down with the VSI")
		}
		vol := att[0].Volume.(*vpcv1.VolumeAttachmentPrototypeVolumeVolumePrototypeInstanceContext)
		if got := *vol.Profile.(*vpcv1.VolumeProfileIdentity).Name; got != "general-purpose" {
			t.Errorf("profile = %q, want general-purpose", got)
		}
		if got := *vol.Capacity; got != 60 {
			t.Errorf("capacity = %d, want 60", got)
		}
		if vol.Iops != nil {
			t.Errorf("Iops = %d, want nil (unset)", *vol.Iops)
		}
		if vol.Bandwidth != nil {
			t.Errorf("Bandwidth = %d, want nil (unset)", *vol.Bandwidth)
		}
	})

	// iops and bandwidth are set from two independent blocks, so verify each is
	// honored on its own (guards against a copy-paste bug coupling the two).
	t.Run("custom profile with iops only", func(t *testing.T) {
		att := dataVolumeAttachments(&Config{
			VSIDataCapacity: 60,
			VSIDataProfile:  "custom",
			VSIDataIops:     5000,
		})
		vol := att[0].Volume.(*vpcv1.VolumeAttachmentPrototypeVolumeVolumePrototypeInstanceContext)
		if got := *vol.Profile.(*vpcv1.VolumeProfileIdentity).Name; got != "custom" {
			t.Errorf("profile = %q, want custom", got)
		}
		if vol.Iops == nil || *vol.Iops != 5000 {
			t.Errorf("Iops = %v, want 5000", vol.Iops)
		}
		if vol.Bandwidth != nil {
			t.Errorf("Bandwidth = %d, want nil (unset)", *vol.Bandwidth)
		}
	})

	t.Run("sdp profile with bandwidth only", func(t *testing.T) {
		att := dataVolumeAttachments(&Config{
			VSIDataCapacity:  60,
			VSIDataProfile:   "sdp",
			VSIDataBandwidth: 2000,
		})
		vol := att[0].Volume.(*vpcv1.VolumeAttachmentPrototypeVolumeVolumePrototypeInstanceContext)
		if vol.Bandwidth == nil || *vol.Bandwidth != 2000 {
			t.Errorf("Bandwidth = %v, want 2000", vol.Bandwidth)
		}
		if vol.Iops != nil {
			t.Errorf("Iops = %d, want nil (unset)", *vol.Iops)
		}
	})

	t.Run("custom profile with iops and bandwidth", func(t *testing.T) {
		att := dataVolumeAttachments(&Config{
			VSIDataCapacity:  60,
			VSIDataProfile:   "custom",
			VSIDataIops:      10000,
			VSIDataBandwidth: 2000,
		})
		vol := att[0].Volume.(*vpcv1.VolumeAttachmentPrototypeVolumeVolumePrototypeInstanceContext)
		if got := *vol.Profile.(*vpcv1.VolumeProfileIdentity).Name; got != "custom" {
			t.Errorf("profile = %q, want custom", got)
		}
		if vol.Iops == nil || *vol.Iops != 10000 {
			t.Errorf("Iops = %v, want 10000", vol.Iops)
		}
		if vol.Bandwidth == nil || *vol.Bandwidth != 2000 {
			t.Errorf("Bandwidth = %v, want 2000", vol.Bandwidth)
		}
	})
}
