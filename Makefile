##########################################################
# CONTAINER INFORMATION
NAMESPACE      := ibmcloud
APP_NAME       := packer-plugin-ibmcloud
APP_VERSION    := 2.0.1
CONTAINER_NAME := $(NAMESPACE)/$(APP_NAME):$(APP_VERSION)
WORKDIR        := packer-plugin-ibmcloud
##########################################################


##########################################################
# GO AND PACKER VERSION
GO_VERSION := 1.16.4
PACKER_VERSION := 1.7.3

# DOCKER BUILD ARGUMENTS
DOCKER_BUILD_ARG = --build-arg GO_VERSION=$(GO_VERSION) \
  	--build-arg PACKER_VERSION=$(PACKER_VERSION)
##########################################################


##########################################################
# ENVIRONMENT VARIABLES
CREDENTIALS_FILE          := .credentials
PRIVATE_KEY       				:= /root/.ssh/id_rsa
PUBLIC_KEY        				:= /root/.ssh/id_rsa.pub
ANSIBLE_INVENTORY_FILE    := provisioner/hosts
ANSIBLE_HOST_KEY_CHECKING := False
PACKER_LOG                := 1
PACKER_LOG_PATH           := packerlog/packerlog.txt
OBJC_DISABLE_INITIALIZE_FORK_SAFETY := YES

# DOCKER RUN ENVIRONMENT VARIABLES
DOCKER_RUN_ENV = --env-file=$(CREDENTIALS_FILE) \
		--env PRIVATE_KEY=$(PRIVATE_KEY) \
		--env PUBLIC_KEY=$(PUBLIC_KEY) \
		--env ANSIBLE_INVENTORY_FILE=$(ANSIBLE_INVENTORY_FILE) \
		--env ANSIBLE_HOST_KEY_CHECKING=$(ANSIBLE_HOST_KEY_CHECKING) \
		--env PACKER_LOG=$(PACKER_LOG) \
		--env PACKER_LOG_PATH=$(PACKER_LOG_PATH) \
		--env OBJC_DISABLE_INITIALIZE_FORK_SAFETY=$(OBJC_DISABLE_INITIALIZE_FORK_SAFETY)
##########################################################


##########################################################
# PACKER_TEMPLATE it's passed from command line 
# PACKER_TEMPLATE=examples/build.vpc.centos.pkr.hcl
# How to create volume ==>   -v $(PWD)/host/folder/path:/container/folder/path

image:
	docker build $(DOCKER_BUILD_ARG) -t $(CONTAINER_NAME) .
it:
	docker run -v $(PWD)/examples:/$(WORKDIR)/examples -v $(PWD)/packerlog:/$(WORKDIR)/packerlog $(DOCKER_RUN_ENV) -it $(CONTAINER_NAME)
validate:
	docker run -v $(PWD)/examples:/$(WORKDIR)/examples -v $(PWD)/packerlog:/$(WORKDIR)/packerlog --rm $(DOCKER_RUN_ENV) $(CONTAINER_NAME) validate $(PACKER_TEMPLATE)
build:
	docker run -v $(PWD)/examples:/$(WORKDIR)/examples -v $(PWD)/packerlog:/$(WORKDIR)/packerlog --rm $(DOCKER_RUN_ENV) $(CONTAINER_NAME) build $(PACKER_TEMPLATE)
##########################################################