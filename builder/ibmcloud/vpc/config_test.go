package vpc

import (
	"strings"
	"testing"
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
