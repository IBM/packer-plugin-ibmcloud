setup:
	cd ..; go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@latest
	cd ..; go get -d github.com/hashicorp/hcl/v2/hcldec@latest
	cd ..; go get -d golang.org/x/crypto/ssh@latest
	cd ..; go get -d github.com/zclconf/go-cty/cty@latest

plugin:
	cd ..; go mod tidy
	cd ..; go mod vendor
	cd ..; go generate ./builder/ibmcloud/...
	cd ..; go mod vendor
	cd ..; go build .

run-jowko:
	cd ..; packer validate developer/examples/jowko.pkr.hcl
	cd ..; packer build developer/examples/jowko.pkr.hcl

run-centos:
	cd ..; packer validate developer/examples/build.vpc.centos.pkr.hcl
	cd ..; packer build developer/examples/build.vpc.centos.pkr.hcl	

run-init-centos:
	cd ..; packer init -upgrade developer/examples/build.vpc.init-centos.pkr.hcl
	cd ..; packer validate developer/examples/build.vpc.init-centos.pkr.hcl
	cd ..; packer build developer/examples/build.vpc.init-centos.pkr.hcl	

run-windows:
	# cd ..; packer init -upgrade examples/build.vpc.windows.pkr.hcl
	cd ..; packer validate examples/build.vpc.windows.pkr.hcl	
	cd ..; packer build examples/build.vpc.windows.pkr.hcl	
	cd ..; packer build examples/build.vpc.windows.json