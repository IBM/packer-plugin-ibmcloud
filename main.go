package main

import (
	"log"

	"github.com/hashicorp/packer/packer/plugin"
	"github.com/ibmcloud/packer-plugin-ibmcloud/builder/ibmcloud"
	"github.com/ibmcloud/packer-plugin-ibmcloud/version"
)

func main() {
	log.Println("IBM Cloud Provider version", version.FormattedVersion, version.VersionPrerelease, version.GitCommit)
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}
	server.RegisterBuilder(new(ibmcloud.Builder))
	server.Serve()
}
