packer {
  required_plugins {
    ibmcloud = {
      version = ">=v2.2.0"
      source  = "github.com/IBM/ibmcloud"
    }
  }
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

source "ibmcloud-vpc" "windows" {
  api_key = "${var.ibm_api_key}"
  region  = "au-syd"

  subnet_id         = "02h7-9645d633-55a8-463c-b3b3-5cd302f2ee32"
  resource_group_id = ""
  security_group_id = ""

  vsi_base_image_id  = "r026-0b7a41fa-4d00-44bb-b3ab-e8c1ed04d4ad"
  vsi_profile        = "bx2-2x8"
  vsi_interface      = "public"
  vsi_user_data_file = "scripts/winrm_setup.ps1"

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
    inventory_file = "${var.ansible_inventory_file}"
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
