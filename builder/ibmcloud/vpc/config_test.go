package vpc

import (
	"strings"
	"testing"
)

// otherwise-valid config so Prepare surfaces only the boot-capacity check.
func validBootCapacityConfig() *Config {
	c := &Config{}
	c.Comm.Type = "ssh"
	c.IBMApiKey = "test-api-key"
	c.Region = "us-east"
	c.SubnetID = "0717-test-subnet"
	c.VSIBaseImageID = "r014-test-image"
	c.VSIProfile = "bz2-4x16"
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
			c := validBootCapacityConfig()
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
