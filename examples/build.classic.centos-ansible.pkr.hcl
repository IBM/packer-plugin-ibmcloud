packer {
  required_plugins {
    ibmcloud = {
      version = ">=v2.2.0"
      source  = "github.com/IBM/ibmcloud"
    }
  }
}

variable "unique-id" {
  type    = string
  default = "ibmcloud-cl"
}

variable "sl_username" {
  type    = string
  default = "${env("SL_USERNAME")}"
}

variable "sl_api_key" {
  type    = string
  default = "${env("SL_API_KEY")}"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-classic" "centos" {
  api_key  = "${var.sl_api_key}"
  username = "${var.sl_username}"

  instance_name          = "${var.unique-id}-vsi"
  base_image_id          = "5586378f-5f4f-4eac-9aa2-199fa53bb15e"
  datacenter_name        = "dal12"
  instance_domain        = "${var.unique-id}.com"
  instance_cpu           = 2
  instance_memory        = 4096
  instance_network_speed = 10
  instance_disk_capacity = 25
  instance_state_timeout = "25m"

  communicator = "ssh"
  ssh_port     = 22
  ssh_timeout  = "15m"
  ssh_username = "root"

  image_name        = "packer-${local.timestamp}"
  image_description = "Centos image created by ibmcloud packer plugin at {{isotime}}"
  image_type        = "standard"
  upload_to_datacenters = [
    "352494"
  ]
}

build {
  sources = [
    "source.ibmcloud-classic.centos"
  ]

  provisioner "shell" {
    execute_command = "{{.Vars}} bash '{{.Path}}'"
    inline = [
      "yum install -y dnsmasq"
    ]
  }

  provisioner "ansible" {
    playbook_file = "provisioner/centos-playbook.yml"
  }

}
