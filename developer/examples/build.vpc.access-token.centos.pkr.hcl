// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v3.0.0"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }

// IAM Service API Key authentication example — use this when you have a Service
// ID API key and need the plugin to perform both token-exchange steps internally.

variable "IAM_SERVICE_API_KEY" {
  type      = string
  sensitive = true
}

variable "DESIRED_IAM_ID" {
  type = string
}

variable "SUBNET_ID" {
  type = string
}

variable "REGION" {
  type = string
}

variable "RESOURCE_GROUP_ID" {
  type    = string
  default = ""
}

variable "SECURITY_GROUP_ID" {
  type    = string
  default = ""
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-vpc" "centos" {

  iam_service_api_key = var.IAM_SERVICE_API_KEY
  desired_iam_id      = var.DESIRED_IAM_ID

  region = var.REGION

  subnet_id         = var.SUBNET_ID
  resource_group_id = var.RESOURCE_GROUP_ID
  security_group_id = var.SECURITY_GROUP_ID

  vsi_base_image_name = "ibm-centos-stream-10-amd64-2"
  vsi_profile         = "bx2-2x8"
  vsi_interface       = "public"

  image_name = "packer-${local.timestamp}"

  communicator = "ssh"
  ssh_username = "vpcuser"
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
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure' >> /tmp/hello.txt"
    ]
  }
}
