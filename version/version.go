package version

import (
	"github.com/hashicorp/packer-plugin-sdk/version"
)

var IBMCloudPluginVersion *version.PluginVersion

var (
	// Version is the main version number that is being run at the moment.
	Version           = "v2.0.1"
	VersionPrerelease = "dev"
)

func init() {
	IBMCloudPluginVersion = version.InitializePluginVersion(Version, VersionPrerelease)
}
