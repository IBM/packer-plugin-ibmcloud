// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v3.0.3"
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

source "ibmcloud-vpc" "centos-encrypted-image" {
  api_key           = var.IBM_API_KEY
  region            = var.REGION
  subnet_id         = var.SUBNET_ID
  resource_group_id = var.RESOURCE_GROUP_ID
  security_group_id = var.SECURITY_GROUP_ID
  encryption_key_crn = "crn:v1:bluemix:public:kms:us-south:a/7f75c7b025e54bc5635f754b2f888665:e33a2dc9-63b2-49d5-9a14-abaedbf0a9bf:key:ed000596-f68b-46df-a568-1ad866fbaebe"

  vsi_base_image_name = "test-encrypted-packer-img"

  vsi_profile        = "bx2-2x8"
  vsi_interface      = "public"
  vsi_user_data_file = ""

  image_name = "packer-encrypted-image-${local.timestamp}"

  communicator = "ssh"
  ssh_username = "root"
  ssh_port     = 22
  ssh_timeout  = "15m"

  timeout = "30m"
}

build {
  sources = [
    "source.ibmcloud-vpc.centos-encrypted-image"
  ]

  provisioner "shell" {
    execute_command = "{{.Vars}} bash '{{.Path}}'"
    inline = [
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure'",
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure' >> /hello.txt"
    ]
  }
}
