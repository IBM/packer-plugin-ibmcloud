# IBM Packer Plugin

## Scope
The IBM Packer Plugin can be used to create custom Images on IBM Cloud.

## Description
IBM Packer Plugin adds on two **Packer Builders**: one for *Classic Infrastructure* and one for *VPC Infrastructure*. A **Packer Builder** is a Packer component responsible for creating a machine image. A Builder reads in a **Packer Template**, a configuration file that defines the image you want to build and how to build it. From this configuration file the Builder takes a source OS image (Linux or Windows) and provisions a VSI. Then, the **Builder** installs software for your specific use-case and generates an Image out of the VSI. This generated Image can be reused to launch new VSI Instances within IBM Cloud.

### Builders
- [classic](builders/classic) - The `classic` builder support the creation of custom Images(.VHD) on IBM Cloud - Classic Infrastructure.
- [vpc](builders/vpc) - The `vpc` builder support the creation of custom Images on IBM Cloud - VPC Infrastructure.

### Prerequisites
- Install [Packer](https://www.packer.io/downloads) >= 1.7
- Install [Ansible](https://docs.ansible.com/ansible/latest/installation_guide/intro_installation.html#installing-ansible-on-specific-operating-systems) >= 2.10, if Ansible is your preferred Provisioner (recommended).
- Install [Go](https://golang.org/doc/install) >= 1.17, if you want to use `Manual Installation`. Environment variables for golang setup.
  ```shell
  export GOPATH=$HOME/go
  export GOROOT=/usr/local/go
  export PATH=$PATH:$GOPATH/bin:$GOROOT/bin
  export PACKERPATH=/usr/local/packer
  export PATH=$PATH:$PACKERPATH
  ```
- For Windows Image - Install python package for winrm
  ```shell
  pip3 install --ignore-installed "pywinrm>=0.2.2" --user
  ```
- Create `.env` file and set IBM Cloud Credentials. Also, set Packer and Ansible environment variables.
  ```shell
  # VPC
  export IBM_API_KEY=""
  # Classic
  export SL_USERNAME=""
  export SL_API_KEY=""

  export ANSIBLE_INVENTORY_FILE="provisioner/hosts"
  export ANSIBLE_HOST_KEY_CHECKING=False
  export PACKER_LOG=1
  export PACKER_LOG_PATH="packerlog/packerlog.txt"
  export OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES
  ```

## Usage
### Using the `packer init` command
Starting from version 1.7, Packer supports third-party plugin installation using `packer init` command. Read the
[Packer documentation](https://www.packer.io/docs/commands/init) for more information.

1. `packer init` downloads Packer Plugin binaries required in your Packer Template. To install a Packer Plugin just copy and paste the `required_plugins` Block inside your Packer Template.

    ```hcl
    packer {
      required_plugins {
        ibmcloud = {
          version = ">=v3.0.0"
          source = "github.com/IBM/ibmcloud"
        }
      }
    }
    ```
    Then run `packer init -upgrade examples/build.vpc.centos.pkr.hcl`

    **Note:**
    - Be aware that `packer init` does not work with legacy JSON templates. Upgrade your JSON config files to HCL. You can find examples on how to do it at `developer/examples` folder.
    - Plugin will be installed on `$HOME/.packer.d/plugins`

2. Create Configuration files and folders
    - Create preferred folder. i.e.
      `mkdir $HOME/packer-plugin-ibmcloud/`
    - Copy Packer Templates examples folder
      `cp -r examples $HOME/packer-plugin-ibmcloud/`
    - Copy Windows-based VSI config scripts folder:
      `cp -r scripts $HOME/packer-plugin-ibmcloud/`
    - Copy ansible playbooks folder:
      `cp -r provisioner $HOME/packer-plugin-ibmcloud/`
    - Create Packer log folder (recall env variable `PACKER_LOG_PATH`)
      `cp -r packerlog $HOME/packer-plugin-ibmcloud/`

3. Run `source` command to read and execute commands from the `.env` file
    ```shell
    source .env
    ```

4. Finally, run Packer plugin commands
    ```shell
    packer validate examples/build.vpc.centos.pkr.hcl
    packer build examples/build.vpc.centos.pkr.hcl
    ```

***********

## Packer Template in detail
Packer's behavior is determined by the Packer template. This template tells Packer not only the plugins (builders, provisioners, post-processors) to use, but also how to configure them and in what order run them.

Historically, Packer has used a JSON template for its configuration. From version 1.7.0, HCL2 becomes officially the preferred template configuration format. You can find examples on how to use HCL Templates at `/examples` folder.

This is a basic Packer Template used to create a custom CentOS image on IBM Cloud - VPC

```hcl
packer {
  required_plugins {
    ibmcloud = {
      version = ">=v3.0.0"
      source = "github.com/IBM/ibmcloud"
    }
  }
}

variable "ibm_api_key" {
  type = string
  default = "${env("IBM_API_KEY")}"
}

locals {
  timestamp = regex_replace(timestamp(), "[- TZ:]", "")
}

source "ibmcloud-vpc" "centos" {
  api_key = "${var.ibm_api_key}"
  region = "au-syd"

  subnet_id = "02h7-9645d633-55a8-463c-b3b3-5cd302f2ee32"
  resource_group_id = ""
  security_group_id = ""

  vsi_base_image_name = "ibm-centos-8-3-minimal-amd64-3"
  vsi_profile = "bx2-2x8"
  vsi_interface = "public"
  vsi_user_data_file = "scripts/postscript.sh"
  image_name = "packer-${local.timestamp}"

  communicator = "ssh"
  ssh_username = "root"
  ssh_port = 22
  ssh_timeout = "15m"
  timeout = "30m"
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
```
### Understanding Packer Template Blocks

#### `variable` Block
The `variable` block defines variables within your Packer configuration. Input variables serve as parameters for a Packer build, allowing aspects of the build to be customized without altering the build's own source code. When you declare variables in the build of your configuration, you can set their values using CLI options and environment variables.

#### `local` Block
The `local` block defines exactly one local variable within a folder. Local values assign a name to an expression, that can then be used multiple times within a folder.

#### `packer` Block
The `packer` configuration block type is used to configure some behaviors of Packer itself, such as its source and the minimum required Packer version needed to apply your configuration.

#### `build` Block
The `build` block defines what builders are started, how to provision them and if necessary what to do with their `artifacts` using post-process.
- A `source` block nested in a `build` block allows you to use an already defined source and to "fill in" those fields which aren't already set in the top-level source block.
- The `provisioner` block defines how a provisioner is configured. Provisioners use builtin and third-party software to install and configure the machine image after booting. Provisioners prepare the system for use. Common use cases for provisioners include: installing packages, patching the kernel, creating users or downloading application code.


#### `source` block
The top-level `source` block defines reusable builder configuration blocks.
```hcl
source "ibmcloud" "vpc-centos" {
   ...
```
Variable | Type |Description
--- | --- | ---
**builder variables** |
type | string | Set it as "ibmcloud"
| |
api_key | string | The IBM Cloud platform API key. Required.
region | string | IBM Cloud region where VPC is deployed. Required.
subnet_id | string | The VPC Subnet identifier. Required.
resource_group_id | string | The resource group identifier to use. If not specified, IBM packer plugin uses `default` resource group.
security_group_id | string | The security group identifier to use. If not specified, IBM packer plugin creates a new temporary security group to allow SSH and WinRM access.
| |
vsi_base_image_id | string | The base image identifier used to created the VSI. Use `ibmcloud is images` for available options.
| OR |
vsi_base_image_name | string | The base image name used to created the VSI. Use `ibmcloud is images` for available options.
| |
vsi_profile | string | The profile this VSI uses. Required.
vsi_interface | string | Set it as "public" to create a Floating IP to connect to the temp VSI. Set it as "private" to use private interface to connect to the temp VSI. Later option requires you run packer plugin inside your VPC.
| |
| |
vsi_user_data_file | string | User data to be made available when setting up the virtual server instance. Optional.
vpc_endpoint_url | string | Configure URL for VPC test environments. Optional.
iam_url | string | Configure URL for IAM test environments. Optional.
image_name | string | The name of the resulting custom Image that will appear in your account. Required.
communicator | string | Communicators are the mechanism Packer uses to upload files, execute scripts, etc. with the machine being created. Choose between "ssh" (for Linux) and "winrm" (for Windows). Required.
***Linux Communicator Variables*** |
ssh_username | string | The username to connect to SSH with.
ssh_port | int |The port that SSH will be available on. Defaults to port 22.
ssh_timeout | string | The time to wait for SSH to become available before timing out. The format of this value is a duration such as "5s" or "5m".
***Windows Communicator Variables*** |
winrm_username | string | The username to use to connect to WinRM.
winrm_port | int |The port that WinRM will be available on. Defaults to port 5986.
winrm_timeout | string | The time to wait for WinRM to become available before timing out.
winrm_insecure | bool | If true, do not check server certificate chain and host name.
winrm_use_ssl | bool | If true, use HTTPS for WinRM.
| |
timeout | string | The amount of time to wait before considering that the provisioner failed. Optional.

***********

## Security Groups Rules
IBM Packer Plugin - VPC Builder add rules to the Security Group to enable WinRM and SSH communication.

### - Connection to Windows-based VSIs via WinRM
+ Protocol: TCP, Port range: 5985-5986, Source Type: Any

### - Connection to Linux-based VSIs via SSH
+ Protocol: TCP, Port range: 22-22, Source Type: Any

## WinRM Setup
- MUST use `scripts/winrm_setup.ps1` scrips to setup WinRM communication with a Windows VSI's in VPC Infrastructure.

- MUST use `scripts/undo_winrm.ps1` to revert WinRM configuration to a pristine state. Read more about it on [Packer documentation](https://learn.hashicorp.com/tutorials/packer/getting-started-build-image?in=packer/getting-started#a-windows-example)

## Microsoft Remote Desktop
If you want to connect to a Windows-based VSI via Microsoft Remote Desktop, go to VPC Default Security Group and add these two rules:
+ Protocol: TCP, Port range: 3389-3389, Source Type: Any
+ Protocol: UDP, Port range: 3389-3389, Source Type: Any

***********

## Developers

- [Manual Installation](#manual-installation)
- [Automation via Docker Container](#automation-via-docker-container)

### Manual Installation
To generate the packer plugin binary from source code follow these steps. An automation script is located on the folder `developer/Makefile`:
1. Clone the GitHub repo here to your laptop and place the repo at folder `$GOPATH/src/github.com/ibmcloud/packer-plugin-ibmcloud`
2. Next, we need to generate the packer plugin binary by running these commands:
    ```shell
    cd $GOPATH/src/github.com/ibmcloud/packer-plugin-ibmcloud
    go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@latest
    go get -d github.com/hashicorp/hcl/v2/hcldec@latest
    go get -d golang.org/x/crypto/ssh@latest
    go get -d github.com/zclconf/go-cty/cty@v1.9.1
    go mod tidy
    go mod vendor
    go generate ./builder/ibmcloud/vpc/...
    go mod vendor
    go build .
    ```
    The packer plugin binary is called packer-plugin-ibmcloud and is located at `$GOPATH/src/github.com/ibmcloud/packer-plugin-ibmcloud`
    <br/>
3. Once the packer plugin binary is generated, copy plugin binary and configuration files and folders on a preferred folder:
    - Create preferred folder . i.e.
      `mkdir $HOME/packer-plugin-ibmcloud/`
    - Go to folder
      `cd $GOPATH/src/github.com/ibmcloud/packer-plugin-ibmcloud`
    - Copy packer plugin binary:
      `cp packer-plugin-ibmcloud $HOME/packer-plugin-ibmcloud/`
    - Give execute permission to the packer plugin binary:
      `chmod +x $HOME/packer-plugin-ibmcloud/packer-plugin-ibmcloud`
    - Copy Packer Templates examples folder
      `cp -r examples $HOME/packer-plugin-ibmcloud/`
    - Copy Windows-based VSI config scripts folder:
      `cp -r scripts $HOME/packer-plugin-ibmcloud/`
    - Copy ansible playbooks folder:
      `cp -r provisioner $HOME/packer-plugin-ibmcloud/`
    - Create Packer log folder (recall env variable `PACKER_LOG_PATH`)
      `cp -r packerlog $HOME/packer-plugin-ibmcloud/`
4. Run `source` command to read and execute commands from the `.env` file
    ```shell
    source .env
    ```
5. Finally, run Packer plugin commands
    ```shell
    packer validate examples/build.vpc.centos.pkr.hcl
    packer build examples/build.vpc.centos.pkr.hcl
    ```

<br/>

### Automation via Docker Container
If you prefer an automation way to build the IBM Cloud Packer Plugin from source code, then clone it from GitHub.
There is a `Makefile` and a `Dockerfile` that automate everything for you.

- The `Dockerfile` will create an image with everything on it to run the IBM Cloud Packer Plugin.
- The `Makefile` will setup the environment variables, volumes and run the container.
  - **Optional**: Custom `Makefile` if you want to change default configuration.

1. Create Packer Plugin Binary within the container:
    - Custom `.credentials` file with your IBM Cloud credentials. Avoid using any kind of quotes: ", '.
      ```shell
      # VPC
      IBM_API_KEY=###...###
      # Classic
      SL_USERNAME=###...###
      SL_API_KEY=###....###
      ```
      Or create a file `variables.pkrvars.hcl` with the following content.
      ```
      SUBNET_ID = ""
      REGION = ""
      SECURITY_GROUP_ID = ""
      RESOURCE_GROUP_ID = ""
      IBM_API_KEY = ""
      ```
    - Customize your Packer Template: see [`source` Block in detail](#source-block-in-detail) to find a detail description of each field on the Template. Likewise, there are some Packer Template examples on `examples` folder.
    - Create container with Packer Plugin Binary within it:
      run `make image`

2. Run Packer
    - Validate the syntax and configuration of your Packer Template by running with `.credentials` file:
        ```bash
        $ make validate PACKER_TEMPLATE=developer/examples/build.vpc.centos-ansible.pkr.hcl
        ```
      Or with `variables.pkrvars.hcl` file
        ```bash
        $ make validate PACKER_TEMPLATE=developer/examples/build.vpc.centos-ansible.pkr.hcl PACKER_VARS_FILE=developer/variables.pkrvars.hcl
        ```
      Customize here your `PACKER_TEMPLATE` path.
    - Generate the custom image by running  with `.credentials` file:
        ```bash
        $ make build PACKER_TEMPLATE=developer/examples/build.vpc.centos-ansible.pkr.hcl
        ```
      Or with `variables.pkrvars.hcl` file
        ```bash
        $ make build PACKER_TEMPLATE=developer/examples/build.vpc.centos-ansible.pkr.hcl PACKER_VARS_FILE=developer/variables.pkrvars.hcl`
        ```
      Customize here your `PACKER_TEMPLATE` path.

**Note**
- You only need to create the image once. *Step 1.*
- The volume attached to the container allows you to update local Packer Templates placed at `/examples` folder, without worried about re-create the docker image again. Just run the container when you are ready using *Step 2* above.
- Another advantage is that you can run multiple containers at the same time.


***********
## Open source @ IBM
Find more open source projects on the [IBM Github Page](http://ibm.github.io/).

### Contributing
Any contribution to this project is welcome, so if you want to contribute by adding a new feature or fixing a bug, do so by opening a Pull Request.

#### Formatting

Before you commit any changes to `hcl` files, it is recommended to format them using packer. Example:
```bash
packer fmt examples/.
packer fmt developer/examples/.
```
This helps to maintain consistent formatting across whole repository.

### License

This SDK project is released under the Apache 2.0 license.
The license's full text can be found in [LICENSE](LICENSE).



