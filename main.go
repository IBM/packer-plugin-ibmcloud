package main

import (
	"github.com/softlayer/packer-builder-ibmcloud/builder/ibmcloud"
	"github.com/hashicorp/packer/packer/plugin"
)

func main() {
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}
	server.RegisterBuilder(new(ibmcloud.Builder))
	server.Serve()
}
