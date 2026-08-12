packer {
  required_plugins {
    ibmcloud = {
      version = ">=v3.0.0"
      source  = "github.com/IBM/ibmcloud"
    }
  }
}

variable "ibm_api_key" {
  type    = string
  default = ""
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

# Multi-subnet capacity fallback: supply several candidate subnets (one per zone,
# all in the same VPC). The builder tries them in a random order and moves on to
# the next when a zone has no host capacity for vsi_profile
# (a "cannot_start_capacity" failure). Use this instead of subnet_id when the
# builder profile is scarce in a single zone. Provide exactly one of subnet_id or
# subnet_ids.
source "ibmcloud-vpc" "centos" {
  api_key = "${var.ibm_api_key}"
  region  = "us-east"

  subnet_ids = [
    "0757-4ad0af5f-8084-469d-a10e-49c444caa312", # us-east-1
    "0767-1b2c3d4e-5f60-7182-93a4-b5c6d7e8f901", # us-east-2
    "0777-9a8b7c6d-5e4f-3021-8f9e-1d2c3b4a5968", # us-east-3
  ]
  resource_group_id = "1984ce401571473492918ea987dd1e6f"
  security_group_id = ""

  vsi_base_image_name = "ibm-centos-stream-10-amd64-2"
  vsi_profile         = "bx2-2x8"
  vsi_interface       = "public"
  image_name          = "packer-${local.timestamp}"

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
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure.'",
    ]
  }
}
