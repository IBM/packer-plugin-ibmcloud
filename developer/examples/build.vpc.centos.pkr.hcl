// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v2.2.0"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }

variable "ibm_api_key" {
  type    = string
  default = "${env("IBM_API_KEY")}"
}

variable "subnet_id" {
  type    = string
  default = "${env("SUBNET_ID")}"
}

variable "region" {
  type    = string
  default = "${env("REGION")}"
}

variable "resource_group_id" {
  type    = string
  default = "${env("RESOURCE_GROUP_ID")}"
}

variable "security_group_id" {
  type    = string
  default = "${env("SECURITY_GROUP_ID")}"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-vpc" "centos" {
  api_key = "${var.ibm_api_key}"
  region  = "${var.region}"

  subnet_id         = "${var.subnet_id}"
  resource_group_id = "${var.resource_group_id}"
  security_group_id = "${var.security_group_id}"

  vsi_base_image_name = "ibm-centos-7-9-minimal-amd64-5"

  vsi_profile        = "bx2-2x8"
  vsi_interface      = "public"
  vsi_user_data_file = ""

  image_name = "packer-${local.timestamp}"

  communicator = "ssh"
  ssh_username = "root"
  ssh_port     = 22
  ssh_timeout  = "15m"

  timeout = "30m"
}

build {
  sources = [
    "source.ibmcloud-vpc.centos"
  ]

  provisioner "shell" {
    execute_command = "{{.Vars}} bash '{{.Path}}'"
    inline = [
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure'",
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure' >> /hello.txt"
    ]
  }
}
