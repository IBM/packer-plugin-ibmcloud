// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v3.1.0"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }

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

variable "ansible_inventory_file" {
  type    = string
  default = "${env("ANSIBLE_INVENTORY_FILE")}"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-classic" "windows" {
  api_key  = "${var.sl_api_key}"
  username = "${var.sl_username}"

  instance_name          = "${var.unique-id}-vsi"
  instance_flavor        = "B1_2X4X100"
  datacenter_name        = "wdc01"
  base_image_id          = "336695f9-a795-470a-9b44-a2df49388b04"
  instance_domain        = "${var.unique-id}.com"
  instance_network_speed = 10
  instance_state_timeout = "60m"

  communicator   = "winrm"
  winrm_username = "Administrator"
  winrm_timeout  = "15m"
  winrm_insecure = true
  winrm_use_ssl  = true
  winrm_port     = 5986

  image_name        = "packer-${local.timestamp}"
  image_description = "Windows image created by ibmcloud packer plugin at {{isotime}}"
  image_type        = "standard"
  upload_to_datacenters = [
    "352494"
  ]
}

build {
  sources = [
    "source.ibmcloud-classic.windows"
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