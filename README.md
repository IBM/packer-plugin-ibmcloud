# IBM Packer Plugin
The IBM Packer Plugin can be used to create custom Images on IBM Cloud. There is one Packer Builder for Classic Infrastructure and one Packer Builder for VPC Infrastructure.

## Description
The builder takes a source OS base Linux or Windows image (identified by it's global ID), provisions an Instance, adds additional applications and services to it and generates an Image out of the Instance. This generated Image can be reused to launch new VSI Instances within IBM Cloud.
The builder does not manage Images. Once it creates an Image, it is up to you to use it or delete it.

### Builders
- [classic](builders/classic) - The `classic` builder support the creation of Image template(.VHD) with pre-configured OS and installed softwares on IBM Cloud - Classic Infrastructure. **- Not yet available. Clone `classic` branch instead**
- [vpc](builders/vpc) - The `vpc` builder support the creation of custom Images on IBM Cloud - VPC Infrastructure.


## Installation 
IBM Packer Plugin may be installed in the following ways:

### Manual installation
Retrieve the packer plugin binary by compiling it from source.  
- To install the plugin, please follow the Packer documentation on
[installing a plugin](https://www.packer.io/docs/extending/plugins/#installing-plugins).  


### Using the `packer init` command - Recommended
Starting from version 1.7, Packer supports third-party plugin installation using `packer init` command. Read the
[Packer documentation](https://www.packer.io/docs/commands/init) for more information.

`packer init` Download Packer plugin binaries required in your Packer Template. To install this plugin just copy and paste the `required_plugins` block inside your Packer Template.

```hcl
packer {
  required_plugins {
    ibmcloud = {
      version = ">=v2.0.2"
      source = "github.com/IBM/ibmcloud"
    }
  }
}
```
- Then run  
  `packer init -upgrade examples/build.vpc.centos.pkr.hcl`    
   
  **Note:** Be aware that `packer init` does not work with legacy JSON templates. Upgrade your JSON config files to HCL. Plugin will be installed on `$HOME/.packer.d/plugins`

  <br/>
- Once you have everything ready update `.env` file with your IBM Cloud credentials  
  ``` 
  # VPC   
  export IBM_API_KEY=""
  # or Classic
  export SL_USERNAME=""
  export SL_API_KEY=""

  # Location where temp SSH Keys will be created
  export PRIVATE_KEY="ssh_keys/id_rsa"
  export PUBLIC_KEY="ssh_keys/id_rsa.pub"

  export ANSIBLE_INVENTORY_FILE="provisioner/hosts"
  export ANSIBLE_HOST_KEY_CHECKING=False
  export PACKER_LOG=1
  export PACKER_LOG_PATH="packerlog/packerlog.txt"
  export OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES
  ```     

   
- Finally, run Packer plugin commands  
   ```
   $ source .env

   # Custom the Packer Template file with proper mandatory and optional fields   
   $ packer validate examples/build.vpc.centos.pkr.hcl  
   $ packer build examples/build.vpc.centos.pkr.hcl	
   or
   $ packer validate examples/build.vpc.windows.pkr.hcl	
   $ packer build examples/build.vpc.windows.pkr.hcl
   ```

***********

## Packer Template in detail
Packer's behavior is determined by the Packer template. This template tells Packer not only the plugins (builders, provisioners, post-processors) to use, but also how to configure them and in what order run them.   

Historically, Packer has used a JSON template for its configuration. From version 1.7.0, HCL2 becomes officially the preferred template configuration format. You can find examples on how to use both at `/examples` folder.  

This is a basic Packer Template used to create a custom CentOS image on IBM Cloud - VPC

```
// packer {
//   required_plugins {
//     ibmcloud = {
//       version = ">=v2.0.2"
//       source = "github.com/IBM/ibmcloud"
//     }
//   }
// }

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
  
  vsi_base_image_id = "r026-3b9ba4a3-b3bd-46ac-9ed4-e53823631a6b"
  vsi_profile = "bx2-2x8"
  vsi_interface = "public"
  vsi_user_data_file = ""

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
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure'",  
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure' >> /hello.txt"
    ]
  }
}
```
### Understanding Packer Template Blocks 
For a detail description of Packer Template configuration [here](https://www.packer.io/docs/templates).  

#### `variable` block
The `variable` block defines variables within your Packer configuration. Input variables serve as parameters for a Packer build, allowing aspects of the build to be customized without altering the build's own source code. When you declare variables in the build of your configuration, you can set their values using CLI options and environment variables.  
 

#### `local` block
The `local` block defines exactly one local variable within a folder. Local values assign a name to an expression, that can then be used multiple times within a folder.


#### `packer` block
The `packer` configuration block type is used to configure some behaviors of Packer itself, such as its source and the minimum required Packer version needed to apply your configuration.


#### `source` block
The top-level `source` block defines reusable builder configuration blocks. 
```
source "ibmcloud" "vpc-centos" {
   ...
```   
You can start builders by refering to those source blocks from a `build` block.  
```
build {
  sources = [
    "source.ibmcloud.vpc-centos"
  ]
```

#### `build` block
The `build` block defines what builders are started, how to provision them and if necessary what to do with their `artifacts` using post-process.
- A `source` block nested in a `build` block allows you to use an already defined source and to "fill in" those fields which aren't already set in the top-level source block.
- The `provisioner` block defines how a provisioner is configured. Provisioners use builtin and third-party software to install and configure the machine image after booting. Provisioners prepare the system for use. Common use cases for provisioners include: installing packages, patching the kernel, creating users, downloading application code, Here we use the `shell` provisioner: the `shell` provisioner provisions machines built by Packer using shell scripts. Shell provisioning is the easiest way to get software installed and configured on a machine.

```
build {
  sources = [
    "source.ibmcloud.vpc-centos"
  ]

  provisioner "shell" {
    execute_command = "{{.Vars}} bash '{{.Path}}'"
    inline = [
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure'",  
      "echo 'Hello from IBM Cloud Packer Plugin - VPC Infrastructure' >> /hello.txt"
    ]
  }
}
```
#### `source` block in Detail
Variable | Type |Description
--- | --- | ---
**builder variables** |
type | string | Set it as "ibmcloud"
api_key | string | The IBM Cloud platform API key
region | string | IBM Cloud region where VPC is deployed
| |
subnet_id | string | The VPC Subnet identifier. Required
resource_group_id | string | The resource group identifier to use. If not specified, IBM packer plugin uses `default` resource group.
security_group_id | string | The security group identifier to use. If not specified, IBM packer plugin creates a new temporary security group to allow SSH and WinRM access.
| |
vsi_base_image_id | string | The base image identifier used to created the VSI. Use `ibmcloud is images` for available options.
| OR |
vsi_base_image_name | string | The base image name used to created the VSI. Use `ibmcloud is images` for available options.
| |
vsi_profile | string | The profile this VSI uses.
vsi_interface | string | Set it as "public" to create a Floating IP to connect to the temp VSI. Set it as "private" to use private interface to connect to the temp VSI. Later option requires you run packer plugin inside your VPC.
vsi_user_data_file | string | User data to be made available when setting up the virtual server instance
| |
image_name | string | The name of the resulting custom Image that will appear in your account.
| |
communicator | string | Communicators are the mechanism Packer uses to upload files, execute scripts, etc. with the machine being created. Choose between "ssh" (for Linux) and "winrm" (for Windows)
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
timeout | string | The amount of time to wait before considering that the provisioner failed.


***********

## Security Groups Rules
IBM Packer plugin add rules to the Security Group to enable WinRM and SSH communication.

### Connection to Windows-based VSIs via WinRM 
+ Protocol: TCP, Port range: 5985-5986, Source Type: Any

### Connection to Linux-based VSIs via SSH
+ Protocol: TCP, Port range: 22-22, Source Type: Any

## Connection to Windows-based VSI via Microsoft Remote Desktop.
If you want to connect to a Windows-based VSI via Microsoft Remote Desktop, go to VPC Default Security Group and add these two rules:
+ Protocol: TCP, Port range: 3389-3389, Source Type: Any
+ Protocol: UDP, Port range: 3389-3389, Source Type: Any

***********

## WinRM Setup
- `winrm_setup.ps1` and `undo_winrm.ps1` scrips are required on Windows vsi's in order to use WinRM

- According with Packer documentation [here](https://learn.hashicorp.com/tutorials/packer/getting-started-build-image?in=packer/getting-started#a-windows-example): "Please note that if you're setting up WinRM for provisioning, you'll probably want to turn it off or restrict its permissions as part of a shutdown script at the end of Packer's provisioning process. For more details on the why/how, check out this useful blog post and the associated [code](https://cloudywindows.io/post/winrm-for-provisioning---close-the-door-on-the-way-out-eh/)"   
   
   IBM Packer plugin ensures to revert WinRM configuration to a pristine state running the script scripts/undo_winrm.ps1 on the section provisioners.


***********

## Developers
In case you want to contribute to the project there is a folder called `developer` with a script to create the IBM Packer Plugin binary from source code. Likewise, there are more Packer Templates examples in both HCL and its equivalent on JSON format. Finally, we have an automation via Docker containers to create the IBM Packer Plugin binary.

### Automation via Docker Container
If you prefer an automation way to build the IBM Cloud Packer Plugin from source code, then clone it from GitHub. 
There is a `Makefile` and a `Dockerfile` that automate everything for you.

- The `Dockerfile` will create an image with everything on it to run the IBM Cloud Packer Plugin.
- The `Makefile` will setup the environment variables, volumes and run the container.  
  - **Optional**: Custom `Makefile` if you want to change default configuration.

#### 1. Create Packer Plugin Binary within the container:
- Custom `.credentials` file with your IBM Cloud credentials. Avoid using any kind of quotes: ", '.
   ```
   # VPC
   IBM_API_KEY=###...###
   # Classic
   SL_USERNAME=###...###
   SL_API_KEY=###....###
   ```
- Customize your Packer Template: see [Configuration](#configuration) to find a detail description of each field on the Template. Likewise, there are some Packer Template examples on `examples` folder. 
- Create container with Packer Plugin Binary within it:
  run `make image`  

#### 2. Run Packer 
- Validate the syntax and configuration of your Packer Template by running:   
   `$ make validate PACKER_TEMPLATE=developer/examples/build.vpc.centos-ansible.pkr.hcl`  
   Customize here your `PACKER_TEMPLATE` path.   
- Generate the custom image by running:   
   `$ make build PACKER_TEMPLATE=developer/examples/build.vpc.centos-ansible.pkr.hcl`  
   Customize here your `PACKER_TEMPLATE` path.

**Note**
- You only need to create the image once. *Step 1.*
- The volume attached to the container allows you to update local Packer Templates placed at `/examples` folder, without worried about re-create the docker image again. Just run the container when you are ready using *Step 2* above.
- Another advantage is that you can run multiple containers at the same time.
