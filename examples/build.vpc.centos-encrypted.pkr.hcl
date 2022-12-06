packer {
  required_plugins {
    ibmcloud = {
      version = ">=v3.0.4"
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

source "ibmcloud-vpc" "centos-encrypted-image" {
  api_key           = "${var.ibm_api_key}"
  region            = "us-south"
  subnet_id         = "0717-4ad0af5f-8084-469d-a10e-49c444caa312"
  resource_group_id = ""
  security_group_id = ""
  encryption_key_crn = var.ENCRYPTION_KEY_CRN

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
