// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v3.0.0"
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
// variable "VPC_URL" {
//   type = string
// }
// variable "IAM_URL" {
//   type = string
// }


locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-vpc" "centos" {
  api_key           = var.IBM_API_KEY
  region            = var.REGION
  subnet_id         = var.SUBNET_ID
  resource_group_id = var.RESOURCE_GROUP_ID
  security_group_id = var.SECURITY_GROUP_ID

  vsi_base_image_name = "ibm-centos-7-9-minimal-amd64-5"

  vsi_profile        = "bx2-2x8"
  vsi_interface      = "public"
  vsi_user_data_file = ""
  vsi_user_data = <<-EOT
    #!/bin/bash
    echo 'Hello from user data by deepak' >> /packer-test-by-deepak.txt
    curl https://raw.githubusercontent.com/dvershinin/apt-get-centos/master/apt-get.sh -o /usr/local/bin/apt-get
    chmod 0755 /usr/local/bin/apt-get
    apt-get update
    apt-get install -y apt-transport-https ca-certificates curl software-properties-common
    curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo apt-key add -
    add-apt-repository \
      "deb [arch=amd64] https://download.docker.com/linux/ubuntu \
      $(lsb_release -cs) \
      stable"
    apt-get update
    apt-get install -y docker-ce
    usermod -aG docker ubuntu

    # Install docker-compose
    curl -L https://github.com/docker/compose/releases/download/1.21.0/docker-compose-$(uname -s)-$(uname -m) -o /usr/local/bin/docker-compose
    chmod +x /usr/local/bin/docker-compose

    # READ MORE: https://docs.docker.com/install/linux/docker-ce/ubuntu/
    # To check: grep "cloud-init\[" /var/log/syslog
    #       OR: less /var/log/cloud-init-output.log

    # Manually add user to docker group
    # sudo usermod -aG docker $USER
    EOT
  image_name = "packer-dp-${local.timestamp}"

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
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure'",
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure' >> /hello.txt"
    ]
  }
}
