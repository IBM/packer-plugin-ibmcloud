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

test-release:
	make plugin
	@echo "==> 1. Running pre-release tests (matching .goreleaser.yml hooks)..."
	cd ..; go test -mod=mod ./...
	@echo "==> 2. Describing plugin and resolving API_VERSION (matching release.yml)..."
	cd ..; API_VERSION=$$(go run -mod=mod . describe | jq -r '.api_version'); \
	echo "Resolved API_VERSION: $${API_VERSION}"; \
	if [ -z "$${API_VERSION}" ] || [ "$${API_VERSION}" = "null" ]; then \
		echo "ERROR: Failed to retrieve api_version from plugin descriptor"; exit 1; \
	fi; \
	echo "==> 3. Running plugin compatibility check (make plugin-check)..." && \
	packer-sdc plugin-check packer-plugin-ibmcloud && \
	echo "==> 4. Cross-compiling all release target binaries with GoReleaser flags (matching .goreleaser.yml)..." && \
	MODULE_PATH=$$(go list -m) && \
	VERSION=$$(git describe --tags --abbrev=0 2>/dev/null || echo "v3.7.0") && \
	LDFLAGS="-s -w -X $${MODULE_PATH}/version.Version=$${VERSION} -X $${MODULE_PATH}/version.VersionPrerelease=" && \
	echo "Using MODULE_PATH=$${MODULE_PATH}, VERSION=$${VERSION}, API_VERSION=$${API_VERSION}" && \
	CGO_ENABLED=0 GOOS=linux   GOARCH=amd64 go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=linux   GOARCH=386   go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm   go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=linux   GOARCH=arm64 go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=darwin  GOARCH=amd64 go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=darwin  GOARCH=arm64 go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=windows GOARCH=386   go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=freebsd GOARCH=386   go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm   go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null . && \
	CGO_ENABLED=0 GOOS=freebsd GOARCH=arm64 go build -mod=mod -trimpath -ldflags "$${LDFLAGS}" -o /dev/null .
	@echo "==> 5. Testing smallest validation set..."
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos.pkr.hcl
	@echo "==> Release pre-check test succeeded!"

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

run-ssh-ed25519:
	cd ..; packer validate -var-file="developer/variables.ssh-pkrvars.hcl" developer/examples/build.vpc.centos-ssh-ed25519.pkr.hcl
	cd ..; packer build -var-file="developer/variables.ssh-pkrvars.hcl" developer/examples/build.vpc.centos-ssh-ed25519.pkr.hcl
	
run-centos:
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos.pkr.hcl
	cd ..; packer build -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.centos.pkr.hcl

run-centos-resource-group-name:
	cd ..; packer validate -var-file="developer/variables-resource-group-name.pkrvars.hcl" developer/examples/build.vpc.centos-resource-group-name.pkr.hcl
	cd ..; packer build -var-file="developer/variables-resource-group-name.pkrvars.hcl" developer/examples/build.vpc.centos-resource-group-name.pkr.hcl
	
run-env-centos:
	cd ..; packer validate -var-file="developer/variables.env.pkrvars.hcl" developer/examples/build.vpc.env.centos.pkr.hcl
	cd ..; packer build -var-file="developer/variables.env.pkrvars.hcl" developer/examples/build.vpc.env.centos.pkr.hcl

run-boot-volume:
	cd ..; packer validate -var-file="developer/variables.boot.volume.pkrvars.hcl" developer/examples/build.vpc.boot.volume.pkr.hcl
	cd ..; packer build -var-file="developer/variables.boot.volume.pkrvars.hcl" developer/examples/build.vpc.boot.volume.pkr.hcl

run-boot-snapshot:
	cd ..; packer validate -var-file="developer/variables.boot.snapshot.pkrvars.hcl" developer/examples/build.vpc.boot.snapshot.pkr.hcl	
	cd ..; packer build -var-file="developer/variables.boot.snapshot.pkrvars.hcl" developer/examples/build.vpc.boot.snapshot.pkr.hcl	

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

run-rhel-skip-reboot:
	cd ..; packer validate -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.rhel.pkr.hcl
	cd ..; packer build -var-file="developer/variables.pkrvars.hcl" developer/examples/build.vpc.rhel.skip.reboot.pkr.hcl
