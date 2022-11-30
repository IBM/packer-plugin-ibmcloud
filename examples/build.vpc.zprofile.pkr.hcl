packer {
  required_plugins {
    ibmcloud = {
      version = ">=v2.2.0"
      source = "github.com/IBM/ibmcloud"
    }
  }
}

variable "ENCRYPTION_KEY_CRN" {
  type = string
}

variable "ibm_api_key" {
  type    = string
  default = "${env("IBM_API_KEY")}"
}

variable "REGION" {
  type = string
}

variable "RESOURCE_GROUP_ID" {
  type = string
}

variable "SECURITY_GROUP_ID" {
  type = string
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-vpc" "zprofile" {
  api_key = "${var.ibm_api_key}"
  region  = "us-south"

  subnet_id         = "0717-4ad0af5f-8084-469d-a10e-49c444caa312"
  resource_group_id = ""
  security_group_id = ""

  vsi_base_image_name = "ibm-zos-2-4-s390x-dev-test-wazi-1"
  vsi_profile        = "bz2-2x8"
  vsi_interface      = "public"
  vsi_user_data_file = ""

  image_name = "packer-zprofile-${local.timestamp}"

  communicator = "ssh"
  ssh_username = "ibmuser"
  ssh_port     = 22
  ssh_timeout  = "20m"

  timeout = "20m"
}

build {
  sources = [
    "source.ibmcloud-vpc.zprofile"
  ]

}