packer {
  required_plugins {
    ibmcloud = {
      version = ">=v3.0.4"
      source  = "github.com/IBM/ibmcloud"
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

variable "ansible_inventory_file" {
  type    = string
  default = "${env("ANSIBLE_INVENTORY_FILE")}"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-vpc" "rhel" {
  api_key = "${var.ibm_api_key}"
  region  = "us-south"

  subnet_id         = "0717-4ad0af5f-8084-469d-a10e-49c444caa312"
  resource_group_id = "1984ce401571473492918ea987dd1e6f"
  security_group_id = ""

  // vsi_base_image_id  = "r006-1366d3e6-bf5b-49a0-b69a-8efd93cc225f"
  vsi_base_image_name = "ibm-redhat-8-6-minimal-amd64-1"
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
