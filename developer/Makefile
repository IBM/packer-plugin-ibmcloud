setup:
	cd ..; pip3 install --ignore-installed "pywinrm>=0.2.2"
	cd ..; pip3 install --ignore-installed ansible
	cd ..; go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@latest
	cd ..; go get github.com/hashicorp/hcl/v2/hcldec@latest
	cd ..; go get golang.org/x/crypto/ssh@latest
	cd ..; go get github.com/zclconf/go-cty/cty@v1.9.1

plugin:
	cd ..; go mod tidy
	cd ..; go mod vendor
	cd ..; go generate ./builder/ibmcloud/...
	cd ..; go generate ./post-processor/ibmcloud-export-image/...
	cd ..; go mod vendor
	cd ..; go build .

validate-all:
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos.pkr.hcl
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos-ansible.pkr.hcl
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.windows.pkr.hcl
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.rhel.pkr.hcl
	cd ..; packer validate -var-file="developer/variables.boot.volume.pkrvars.hcl" developer/examples/build.vpc.boot.volume.pkr.hcl
	cd ..; packer validate -var-file="developer/variables-encrypted.pkrvars.hcl" developer/examples/build.vpc.centos-encrypted.pkr.hcl
	cd ..; packer validate -var-file="developer/variables-catalog-offering.pkrvars.hcl" developer/examples/build.vpc.catalogimage-offering.pkr.hcl
	cd ..; packer validate -var-file="developer/variables-catalog-version.pkrvars.hcl" developer/examples/build.vpc.catalogimage-version.pkr.hcl

run-catalog:
	cd ..; packer validate -var-file="developer/variables-catalog-offering.pkrvars.hcl" developer/examples/build.vpc.catalogimage-offering.pkr.hcl
	cd ..; packer build -var-file="developer/variables-catalog-offering.pkrvars.hcl" developer/examples/build.vpc.catalogimage-offering.pkr.hcl
	cd ..; packer validate -var-file="developer/variables-catalog-version.pkrvars.hcl" developer/examples/build.vpc.catalogimage-version.pkr.hcl
	cd ..; packer build -var-file="developer/variables-catalog-version.pkrvars.hcl" developer/examples/build.vpc.catalogimage-version.pkr.hcl
	
run-catalog-version-crn:
	cd ..; packer validate -var-file="developer/variables-catalog-version.pkrvars.hcl" developer/examples/build.vpc.catalogimage-version.pkr.hcl
	cd ..; packer build -var-file="developer/variables-catalog-version.pkrvars.hcl" developer/examples/build.vpc.catalogimage-version.pkr.hcl
	
run-catalog-offering-crn:
	cd ..; packer validate -var-file="developer/variables-catalog-offering.pkrvars.hcl" developer/examples/build.vpc.catalogimage-offering.pkr.hcl
	cd ..; packer build -var-file="developer/variables-catalog-offering.pkrvars.hcl" developer/examples/build.vpc.catalogimage-offering.pkr.hcl
	
run-centos:
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos.pkr.hcl
	cd ..; packer build -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos.pkr.hcl

run-centos-resource-group-name:
	cd ..; packer validate -var-file="developer/variables-resource-group-name.pkrvars.hcl" developer/examples/build.vpc.centos-resource-group-name.pkr.hcl
	cd ..; packer build -var-file="developer/variables-resource-group-name.pkrvars.hcl" developer/examples/build.vpc.centos-resource-group-name.pkr.hcl

run-boot-volume:
	cd ..; packer validate -var-file="developer/variables.boot.volume.pkrvars.hcl" developer/examples/build.vpc.boot.volume.pkr.hcl
	cd ..; packer build -var-file="developer/variables.boot.volume.pkrvars.hcl" developer/examples/build.vpc.boot.volume.pkr.hcl
	
run-centos-parallel:
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos-parallel.pkr.hcl
	cd ..; packer build -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos-parallel.pkr.hcl

run-centos-ansible:
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos-ansible.pkr.hcl
	cd ..; packer build -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos-ansible.pkr.hcl

run-centos-encrypted:
	cd ..; packer validate -var-file="developer/variables-encrypted.pkrvars.hcl" developer/examples/build.vpc.centos-encrypted.pkr.hcl
	cd ..; packer build -var-file="developer/variables-encrypted.pkrvars.hcl" developer/examples/build.vpc.centos-encrypted.pkr.hcl

run-rhel:
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.rhel.pkr.hcl
	cd ..; packer build -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.rhel.pkr.hcl

run-windows:
	cd ..; packer init -upgrade developer/examples/build.vpc.windows.pkr.hcl
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.windows.pkr.hcl
	cd ..; packer build -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.windows.pkr.hcl

run-zprofile:
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.zprofile.pkr.hcl
	cd ..; packer build -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.zprofile.pkr.hcl

run-rhel-post-processor:
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/postprocessor.vpc.rhel.pkr.hcl
	cd ..; packer build -var-file="developer/variables.pkrvars.hcl" developer/examples/postprocessor.vpc.rhel.pkr.hcl
