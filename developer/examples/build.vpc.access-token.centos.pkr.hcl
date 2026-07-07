// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v3.0.0"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }

// IAM token exchange example — use this when you have an existing IBM Cloud
// access token (e.g. from a CI/CD system or federated login) and cannot supply
// an API key directly.
//
// The plugin exchanges your access token for a scoped token tied to the
// desired_iam_id CRN on every VPC service initialisation, so the token is
// refreshed automatically mid-build without any manual intervention.
//
// Pass credentials via the vars file:
//   packer build -var-file=developer/variables-access-token.pkrvars.hcl \
//     developer/examples/build.vpc.access-token.centos.pkr.hcl

variable "IAM_ACCESS_TOKEN" {
  type      = string
  sensitive = true
}

variable "IAM_DESIRED_IAM_ID" {
  type = string
}

variable "IAM_TOKEN_EXCHANGE_URL" {
  type    = string
  default = "https://iam.cloud.ibm.com/identity/token"
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
  # Authentication — token exchange path (no api_key).
  iam_access_token       = var.IAM_ACCESS_TOKEN
  iam_desired_iam_id     = var.IAM_DESIRED_IAM_ID
  iam_token_exchange_url = var.IAM_TOKEN_EXCHANGE_URL

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
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure (token exchange auth)'",
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure' >> /hello.txt"
    ]
  }
}
