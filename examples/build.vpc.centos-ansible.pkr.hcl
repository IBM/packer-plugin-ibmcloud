packer {
  required_plugins {
    ibmcloud = {
      version = ">=v2.1.1"
      source  = "github.com/IBM/ibmcloud"
    }
  }
}

variable "ibm_api_key" {
  type    = string
  default = "${env("IBM_API_KEY")}"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-vpc" "centos" {
  api_key = "${var.ibm_api_key}"
  region  = "au-syd"

  subnet_id         = "02h7-9645d633-55a8-463c-b3b3-5cd302f2ee32"
  resource_group_id = ""
  security_group_id = ""

  // vsi_base_image_id = "r006-13938c0a-89e4-4370-b59b-55cd1402562d"
  vsi_base_image_name = "ibm-centos-7-9-minimal-amd64-3"
  vsi_profile         = "bx2-2x8"
  vsi_interface       = "public"
  vsi_user_data_file  = "scripts/postscript.sh"
  image_name          = "packer-${local.timestamp}"

  communicator = "ssh"
  ssh_username = "root"
  ssh_port     = 22
  ssh_timeout  = "15m"
  timeout      = "30m"
}

build {
  sources = [
    "source.ibmcloud-vpc.centos"
  ]

  provisioner "shell" {
    execute_command = "{{.Vars}} bash '{{.Path}}'"
    inline = [
      "echo 'Hello from IBM Cloud Packer Plugin'",
      "echo 'Hello from IBM Cloud Packer Plugin' >> /hello.txt"
    ]
  }

  provisioner "ansible" {
    playbook_file = "provisioner/centos-playbook.yml"
  }
}
