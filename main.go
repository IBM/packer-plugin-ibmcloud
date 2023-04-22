package main

import (
	"fmt"
	"log"
	"os"

	"packer-plugin-ibmcloud/version"

	"github.com/hashicorp/packer-plugin-sdk/plugin"

	"packer-plugin-ibmcloud/builder/ibmcloud/classic"
	"packer-plugin-ibmcloud/builder/ibmcloud/vpc"

	ibmcloudexport "packer-plugin-ibmcloud/post-processor/ibmcloud-export"
)

func main() {
	pps := plugin.NewSet()
	pps.RegisterBuilder("vpc", new(vpc.Builder))
	pps.RegisterBuilder("classic", new(classic.Builder))
	pps.RegisterPostProcessor("export", new(ibmcloudexport.PostProcessor))
	pps.SetVersion(version.IBMCloudPluginVersion)
	err := pps.Run()
	log.Println("IBM Cloud Packer Plugin Version", version.IBMCloudPluginVersion)
	if err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}
}
