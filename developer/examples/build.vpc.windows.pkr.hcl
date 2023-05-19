// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v3.0.0"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }


variable "ANSIBLE_INVENTORY_FILE" {
  type    = string
  default = "provisioner/hosts"
}

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

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-vpc" "windows" {
  api_key = var.IBM_API_KEY
  region  = var.REGION

  subnet_id         = var.SUBNET_ID
  resource_group_id = var.RESOURCE_GROUP_ID
  security_group_id = var.SECURITY_GROUP_ID

  vsi_base_image_name = "ibm-windows-server-2019-full-standard-amd64-8"
  vsi_profile         = "bx2-2x8"
  vsi_interface       = "public"
  vsi_user_data_file  = "scripts/winrm_setup.ps1"

  image_name = "packer-${local.timestamp}"

  communicator   = "winrm"
  winrm_username = "Administrator"
  winrm_port     = 5986
  winrm_timeout  = "15m"
  winrm_insecure = true
  winrm_use_ssl  = true

  timeout = "60m"
}

build {
  sources = [
    "source.ibmcloud-vpc.windows"
  ]

  provisioner "powershell" {
    scripts = [
      "scripts/sample_script.ps1"
    ]
    environment_vars = [
      "VAR1=A$Dollar",
      "VAR2=A`Backtick",
      "VAR3=A'SingleQuote",
      "VAR4=DoubleQuote"
    ]
  }

  provisioner "ansible" {
    playbook_file  = "provisioner/windows-playbook.yml"
    use_proxy      = false
    inventory_file = var.ANSIBLE_INVENTORY_FILE
    extra_arguments = [
      "-vvvv",
      "--extra-vars",
      "ansible_user=Administrator ansible_password={{ .WinRMPassword }} ansible_connection=winrm ansible_winrm_server_cert_validation=ignore"
    ]
  }

  provisioner "windows-restart" {
    restart_check_command = "powershell -command \"& {Write-Output 'Machine restarted.'}\""
  }

  provisioner "powershell" {
    scripts = [
      "scripts/undo_winrm.ps1"
    ]
  }

}



