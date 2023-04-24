packer {
  required_plugins {
    ibmcloud = {
      version = ">=v3.0.0"
      source = "github.com/IBM/ibmcloud"
    }
  }
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

source "ibmcloud-vpc" "centos" {
  api_key           = "${var.ibm_api_key}"
  region            = "us-south"
  subnet_id         = "0717-4ad0af5f-8084-469d-a10e-49c444caa312"
  resource_group_id = ""
  security_group_id = ""


  vsi_base_image_name = "ibm-centos-7-9-minimal-amd64-5"

  vsi_profile        = "bx2-2x8"
  vsi_interface      = "public"
  vsi_user_data_file = ""

  image_name = "packer-${local.timestamp}-1"

  communicator = "ssh"
  ssh_username = "root"
  ssh_port     = 22
  ssh_timeout  = "15m"

  timeout = "30m"
}

source "ibmcloud-vpc" "centos-other" {
  api_key           = var.IBM_API_KEY
  region            = var.REGION
  subnet_id         = var.SUBNET_ID
  resource_group_id = var.RESOURCE_GROUP_ID
  security_group_id = var.SECURITY_GROUP_ID

  vsi_base_image_name = "ibm-centos-7-9-minimal-amd64-5"

  vsi_profile        = "bx2-2x8"
  vsi_interface      = "public"
  vsi_user_data_file = ""

  image_name = "packer-${local.timestamp}-2"

  communicator = "ssh"
  ssh_username = "root"
  ssh_port     = 22
  ssh_timeout  = "15m"

  timeout = "30m"
}

build {
  sources = [
    "source.ibmcloud-vpc.centos",
    "source.ibmcloud-vpc.centos-other"
  ]

  provisioner "shell" {
    execute_command = "{{.Vars}} bash '{{.Path}}'"
    inline = [
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure'",
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure' >> /hello.txt"
    ]
  }
}
