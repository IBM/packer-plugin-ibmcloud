// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v2.2.0"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }

variable "IBM_API_KEY" {
  type = string
}

variable "SUBNET_ID" {
  type = string
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
  api_key = var.IBM_API_KEY
  region  = var.REGION

  subnet_id         = var.SUBNET_ID
  resource_group_id = var.RESOURCE_GROUP_ID
  security_group_id = var.SECURITY_GROUP_ID

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