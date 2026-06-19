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

func TestPrepareBootVolumeProfile(t *testing.T) {
	const wantMsg = "profile must be one of"

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
		wantReject bool
	}{
		{"profile without capacity", "sdp", 0, 0, 0, true},
		{"iops without capacity", "sdp", 10000, 0, 0, true},
		{"bandwidth without capacity", "sdp", 0, 4000, 0, true},
		{"profile with capacity", "sdp", 0, 0, 100, false},
		{"nothing set, no capacity", "", 0, 0, 0, false},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := validVPCConfig()
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
