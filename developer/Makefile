setup:
	cd ..; pip3 install --ignore-installed "pywinrm>=0.2.2"
	cd ..; pip3 install --ignore-installed ansible
	cd ..; go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@latest
	cd ..; go get github.com/hashicorp/hcl/v2/hcldec@latest
	cd ..; go get golang.org/x/crypto/ssh@latest
	cd ..; go get github.com/zclconf/go-cty/cty@latest

plugin:
	cd ..; go mod tidy
	cd ..; go mod vendor
	cd ..; go generate ./builder/ibmcloud/...
	cd ..; go mod vendor
	cd ..; go build .

run-jowko:
	cd ..; packer validate developer/examples/jowko.pkr.hcl
	cd ..; packer build developer/examples/jowko.pkr.hcl

run-jowko-classic:
	cd ..; packer validate developer/examples/jowko-classic.pkr.hcl
	cd ..; packer build developer/examples/jowko-classic.pkr.hcl

run-centos:
	cd ..; packer validate developer/examples/build.vpc.centos.pkr.hcl
	cd ..; packer build developer/examples/build.vpc.centos.pkr.hcl

run-centos-ansible:
	cd ..; packer validate developer/examples/build.vpc.centos-ansible.pkr.hcl
	cd ..; packer build developer/examples/build.vpc.centos-ansible.pkr.hcl

run-init-centos:
	cd ..; packer init -upgrade developer/examples/build.vpc.init-centos.pkr.hcl
	cd ..; packer validate developer/examples/build.vpc.init-centos.pkr.hcl
	cd ..; packer build developer/examples/build.vpc.init-centos.pkr.hcl

run-windows:
	cd ..; packer init -upgrade examples/build.vpc.windows.pkr.hcl
	cd ..; packer validate developer/examples/build.vpc.windows.pkr.hcl
	cd ..; packer build developer/examples/build.vpc.windows.pkr.hcl
	# cd ..; packer build developer/examples/build.vpc.windows.json