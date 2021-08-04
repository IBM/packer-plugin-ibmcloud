// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v2.0.1"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }

variable "ibm_api_key" {
  type = string
  default = "${env("IBM_API_KEY")}"
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

source "ibmcloud-vpc" "rhel" {
  api_key = "${var.ibm_api_key}"
  region = "us-east"

  subnet_id = "0757-3b35be95-4bd3-49eb-b99c-d124ea11eef2"
  resource_group_id = "f054d39a43ce4f51afff708510f271cb"
  security_group_id = ""
  
  // ibm-redhat-8-3-minimal-amd64-3                     available    amd64   red-8-amd64                          8.x - Minimal Install 
  vsi_base_image_id = "r014-02843c52-e12b-4f72-a631-931b4bf6589d"
  vsi_profile = "bx2-4x16"
  vsi_interface = "public"
  vsi_user_data_file = ""

  image_name = "packer-vpc-macys-image"

  communicator = "ssh"
  ssh_username = "root"
  ssh_port = 22
  ssh_timeout = "15m"
  
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
