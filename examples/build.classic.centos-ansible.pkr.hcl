// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v2.0.1"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }

variable "unique-id" {
  type = string
  default = "ibmcloud-cl"
}

variable "sl_username" {
  type = string
  default = "${env("SL_USERNAME")}"
}

variable "sl_api_key" {
  type = string
  default = "${env("SL_API_KEY")}"
}

variable "ansible_inventory_file" {
  type = string
  default = "${env("ANSIBLE_INVENTORY_FILE")}"
}

variable "private_key_file" {
  type = string
  default = "${env("PRIVATE_KEY")}"
}

variable "public_key_file" {
  type = string
  default = "${env("PUBLIC_KEY")}"
}

source "ibmcloud-classic" "centos" {
  api_key = "${var.sl_api_key}"
  username = "${var.sl_username}"

  instance_name = "${var.unique-id}-vsi"
  base_image_id = "5586378f-5f4f-4eac-9aa2-199fa53bb15e"
  datacenter_name = "dal12"
  instance_domain = "${var.unique-id}.com"
  instance_cpu = 2
  instance_memory = 4096
  instance_network_speed = 10
  instance_disk_capacity = 25
  instance_state_timeout = "25m"

  communicator = "ssh"
  ssh_port = 22
  ssh_timeout = "15m"
  ssh_username = "root"

  image_name = "${var.unique-id}-image"
  image_description = "Centos image created by ibmcloud packer plugin at {{isotime}}"
  image_type = "standard"
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
    inventory_file = "${var.ansible_inventory_file}"
    ssh_host_key_file = "${var.private_key_file}"
    ssh_authorized_key_file = "${var.public_key_file}"
    sftp_command = "/usr/libexec/openssh/sftp-server"
    extra_arguments = [
      "-vvvv",
      "--extra-vars",
      "ansible_user=root --private-key=${var.private_key_file}"
    ]
  }

}
