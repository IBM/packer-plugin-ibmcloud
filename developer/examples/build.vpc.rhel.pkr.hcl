// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v3.0.4"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }

variable "ENCRYPTION_KEY_CRN" {
  type = string
}

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

source "ibmcloud-vpc" "rhel" {
  api_key = var.IBM_API_KEY
  region  = var.REGION

  subnet_id         = var.SUBNET_ID
  resource_group_id = var.RESOURCE_GROUP_ID
  security_group_id = var.SECURITY_GROUP_ID

  vsi_base_image_name = "ibm-redhat-8-4-minimal-amd64-3"
  vsi_profile         = "bx2-4x16"
  vsi_interface       = "public"
  vsi_user_data_file  = ""

  image_name = "packer-${local.timestamp}"

  communicator = "ssh"
  ssh_username = "root"
  ssh_port     = 22
  ssh_timeout  = "15m"

  timeout = "30m"
}

build {
  sources = [
    "source.ibmcloud-vpc.rhel"
  ]

  provisioner "shell" {
    execute_command = "{{.Vars}} bash '{{.Path}}'"
    inline = [
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure'",
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure' >> /hello.txt"
    ]
  }
}
