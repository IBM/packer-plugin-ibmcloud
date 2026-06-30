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

# Attaches an ephemeral scratch data volume to the builder instance. The data
# volume is deleted with the instance and is NOT part of the captured image
# (the image is captured from the boot volume only). Writing large transient
# build artifacts there — package/module caches, downloads, build trees —
# keeps them off the boot volume so they are not exported at image-capture time,
# which on VPC scales with how much of the boot volume has been written.
source "ibmcloud-vpc" "data-volume" {
  api_key = "${var.ibm_api_key}"
  region  = "us-south"

  subnet_id         = "0717-4ad0af5f-8084-469d-a10e-49c444caa312"
  resource_group_id = "1984ce401571473492918ea987dd1e6f"
  security_group_id = ""

  vsi_base_image_name   = "ibm-ubuntu-24-04-amd64"
  vsi_profile           = "bx2-2x8"
  vsi_boot_vol_capacity = 30
  vsi_interface         = "public"
  image_name            = "packer-${local.timestamp}"

  # Scratch data volume for build caches. Deleted with the builder instance;
  # never captured into the image. sdp lets you pin IOPS independently of size,
  # and is the only profile that also honors a set bandwidth (left default here).
  vsi_data_vol_capacity = 60
  vsi_data_vol_profile  = "sdp"
  vsi_data_vol_iops     = 10000

  communicator = "ssh"
  ssh_username = "root"
  ssh_port     = 22
  ssh_timeout  = "15m"

  timeout = "30m"
}

build {
  sources = [
    "source.ibmcloud-vpc.data-volume"
  ]

  # Mount the scratch volume and point cache/build directories at it BEFORE the
  # heavy provisioners run, so their writes never touch the boot volume. The data
  # volume is the disk that does not hold the root filesystem; format and mount
  # it, then symlink the directories that accumulate large transient data. This
  # assumes exactly one data volume is attached.
  provisioner "shell" {
    execute_command = "{{.Vars}} bash '{{.Path}}'"
    inline = [
      "set -euo pipefail",
      "root_disk=/dev/$(lsblk -no PKNAME \"$(findmnt -no SOURCE /)\")",
      "data_disk=$(lsblk -dpno NAME | grep -vx \"$root_disk\" | head -1)",
      "mkfs.ext4 -q \"$data_disk\"",
      "mkdir -p /scratch && mount \"$data_disk\" /scratch",
      "for d in /root/.cache /var/cache/apt/archives /tmp/build; do mkdir -p \"/scratch$d\" \"$(dirname \"$d\")\"; rm -rf \"$d\"; ln -s \"/scratch$d\" \"$d\"; done",
    ]
  }

  provisioner "shell" {
    execute_command = "{{.Vars}} bash '{{.Path}}'"
    inline = [
      "echo 'Build artifacts written under the relocated cache dirs stay on /scratch and are not captured.'",
    ]
  }
}
