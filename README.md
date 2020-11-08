# IBM Packer Plugin
The IBM packer plugin creates Image template(.VHD) with pre-configured OS and installed softwares on IBMCloud 

## IBM Packer Builder
The builder takes a source OS base Linux or Windows image (identified by it's global ID), provisions an Instance, adds additional applications & services to it and generates an Image Template out of the Instance on different platforms. These generated Images can be reused to launch new VSI Instances within IBMCloud.
The builder does not manage VSI images. Once it creates an image, it is up to you to use it or delete it.

## IBM Packer Provisioner
The provisioners use builtin software or software like ansible to install packages or configure the Image after booting

## IBM Packer Post-Provisoners
Post-processors are optional, and they can be used to upload artifacts.

## Requirements
* [Packer](https://www.packer.io/intro/getting-started/install.html)
* [Go 1.13+](https://golang.org/doc/install)

## Build and Installation  
### Install from release:
   - Download binaries from the [releases page](https://github.com/IBM/packer-plugin-ibmcloud/releases).


### Using a Docker Container  
1. Build the script from Dockerfile   
   `$ docker build -t ibmcloud/packer-plugin-ibmcloud .`   
2. Check image is in the local Docker image registry  
   `$ docker image ls`
3. Run and interact with the container  
   ```
   $ docker run -it ibmcloud/packer-plugin-ibmcloud /bin/bash    
   $ cd $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud
   ```
4. Copy your SSH key pair and give proper permission access.    
   ```
   $ mkdir -p $HOME/.ssh
   @ local
   $ cat $HOME/.ssh/id_rsa | pbcopy
   @ container
   $ vi $HOME/.ssh/id_rsa
   -----BEGIN RSA PRIVATE KEY----- ...

   @ local
   $ cat $HOME/.ssh/id_rsa.pub | pbcopy
   @ container
   $ vi $HOME/.ssh/id_rsa.pub   
   ssh-rsa AAAA ...   

   $ chmod 600 $HOME/.ssh/id_rsa
   $ chmod 600 $HOME/.ssh/id_rsa.pub  
   ```
5. Update .env file with your IBM Cloud credentials  
   ```
   $ vi .env

   export IBM_API_KEY=""
   export ANSIBLE_INVENTORY_FILE="provisioner/hosts"
   export ANSIBLE_HOST_KEY_CHECKING=False
   export PRIVATEKEY="$HOME/.ssh/id_rsa"
   export PUBLICKEY="$HOME/.ssh/id_rsa.pub"
   export PACKER_LOG=1
   export PACKER_LOG_PATH="packerlog.txt"
   export OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES
   ```     
6. Run Packer plugin commands  
   ```
   $ source .env

   # Edit the json file with proper mandatory and optional fields 
   $ packer validate examples/linux.json  or   $ packer validate examples/windows.json
   $ packer build examples/linux.json     or   $ packer build examples/windows.json
   ```



### Install it using a shell script  
1. Go to vsi-setup folder  
   `$ cd vsi-setup`
2. Choose the right installation for your instance: macOS, ubuntu. 
   - Here, setup-ubuntu.sh is used.
3. Copy setup-ubuntu.sh to ubuntu machine.
4. Give permission to setup-ubuntu.sh (Setup plugin on your machine)
   - `chmod +x setup-ubuntu.sh`
5. run `. ./setup-ubuntu.sh`
6. run `go generate ./builder/ibmcloud/...`
7. run `go build`
8. Follow steps 4-6 Using a Docker Container


### Install it manually

1) **Download and install Go**

   https://golang.org/dl/

   https://golang.org/doc/install

   Create your Go Workspace
   ```
   $ mkdir $GOPATH/src/github.com
   ```

   Set environment variables. For example, in MacOS
   ```
   $ export GOROOT=/usr/local/go >> .profile  
   $ export GOPATH=$HOME/go >> .profile   
   ```

2) **Install Packer**

   Download the pre compiled binary from https://www.packer.io/downloads.html/. Unzip it into any directory. After unzipping, you should get the packer binary file. Add the location to the packer binary file to the PATH variable
   ```
   $ export PACKERPATH=/usr/local/packer  
   $ export PATH=$PATH:$GOPATH:$GOROOT/bin:$GOPATH/bin:$PACKERPATH >> .profile  
   ```

   For more instructions on downloading and installing packer, refer https://www.packer.io/docs/install/index.html

   Download Packer dependencies
   ```
   $ go get github.com/hashicorp/packer
   $ cd $GOPATH/src/github.com/hashicorp/packer/vendor
   $ rm -r golang.org
   $ mkdir -p $GOPATH/src/golang.org/x/
   $ cd $GOPATH/src/golang.org/x/
   $ git clone https://go.googlesource.com/crypto  
   $ git clone https://github.com/golang/oauth2.git  
   $ git clone https://go.googlesource.com/net  
   $ git clone https://go.googlesource.com/sys  
   $ git clone https://go.googlesource.com/time  
   $ git clone https://go.googlesource.com/text  
   # below packages are required after change above packages source  
   $ go get github.com/agext/levenshtein  
   $ go get github.com/mitchellh/go-wordwrap  
   $ go get github.com/google/go-cmp/cmp  
   $ mv $GOPATH/src/github.com/hashicorp/packer/vendor/github.com/zclconf $GOPATH/src/github.com  
   $ go get github.com/apparentlymart/go-textseg/textseg  
   $ cd /root/go/src/github.com/apparentlymart/go-textseg  
   $ mkdir v12   
   $ cp -r textseg v12 
   $ cd $GOPATH/src  
   $ go get -u cloud.google.com/go/compute/metadata  
   ```

5) **Setup Ansible** 
   ```
   $ sudo apt update  
   $ sudo apt --yes install software-properties-common  
   $ sudo apt-add-repository --yes --update ppa:ansible/ansible  
   $ sudo apt --yes install ansible  
   
   # Fix "winrm or requests is not installed: No module named winrm"
   $ sudo apt --yes install python-pip  
   $ pip install --ignore-installed "pywinrm>=0.2.2"  
   ```

4) **Permission Enforcement in the SoftLayer API - Update July 2020** 
   Add Compute with Public Network Port: Classic infrastructure > Permissions > Network  
   or  
   ibmcloud sl user permission-edit <user_id> --permission PUBLIC_NETWORK_COMPUTE --enable true


5) **IBM Cloud Packer-Builder**

   Clone this repository 
   ```
   $ mkdir -p $GOPATH/src/github.com/ibmcloud  
   $ cd $GOPATH/src/github.com/ibmcloud  
   
   # main repo
   $ git clone https://github.com/IBM/packer-plugin-ibmcloud.git packer-builder-ibmcloud
   $ cd $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud  

   # Install dependencies for Generate the HCL2 code of a plugin
   $ go get github.com/cweill/gotests/...  
   $ go install github.com/hashicorp/packer/cmd/mapstructure-to-hcl2  
   $ mv $GOPATH/src/github.com/hashicorp/packer/vendor/github.com/hashicorp/hcl $GOPATH/src/github.com/hashicorp  
   $ go generate ./builder/ibmcloud/...
   ```

   Build the plugin
   ```
   $ cd $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud
      
   # make sure you update the version under version/version.go if code has changes/features are added 
   # Eg - current version is 0.1.0. When a new feature added to plugin then the new version should be 0.1.1
   
   $ go build
   ```

   **Important Note - Save your existing SSH keypair(id_rsa and is_rsa.pub) before you run Packer. Ansible provisioner is going to overwrite SSH keypair with its own.**
  
   Create .env file:
   ```
   $ cat $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud/.env
   export SL_USERNAME="devtest@.ibm.com"
   export SL_API_KEY="f940986bdfcc34....7fb50b23e3c77acae"
   export ANSIBLE_INVENTORY_FILE="provisioner/hosts"
   export PRIVATEKEY="$HOME/.ssh/id_rsa" <<<< Specific to linux plugin with ansible support
   export PUBLICKEY="$HOME/.ssh/id_rsa.pub" <<<< Specific to linux plugin with ansible support
   export ANSIBLE_HOST_KEY_CHECKING=False
   export PACKER_LOG=1
   export PACKER_LOG_PATH="packerlog.txt"
   export OBJC_DISABLE_INITIALIZE_FORK_SAFETY=YES  <<<< Specific to MAC client 
   ```

   Run Packer:
   ```
   $ source .env

   # Edit the json file with proper mandatory and optional feilds 
   
   $ packer validate examples/linux.json or examples/windows.json
   $ packer build examples/linux.json or examples/windows.json
   ```

   If you are willing to use your own image as your starting point, you can specify `base_image_id` instead of `base_os_code`.

## Configuration Reference

The reference of available configuration options is listed below.

### Required parameters:

Variable | Type | Description
--- | --- | ---
username | string | The user name to use to access your account. If unspecified, the value is taken from the SOFTLAYER_USER_NAME environment variable.
api_key | string | The api key defined for the chosen user name. You can find what is your api key at the account->users tab of the SoftLayer web console. If unspecified, the value is taken from the SOFTLAYER_API_KEY environment variable.
image_name | string | The name of the resulting image that will appear in your account. This must be unique. To help make this unique, use a function like timestamp.
base_image_id | string | The ID of the base image to use (usually defined by the `globalIdentifier` or the `uuid` fields in SoftLayer API). This is the image that will be used for launching a new instance. To view all of your currently available images, `run: curl -X GET --user <username>:<api_key> "https://api.softlayer.com/rest/v3/SoftLayer_Account/getVirtualDiskImages.json"`
instance_name | string | The name assigned to the instance.
instance_flavor | string | The flavor to opt for the instance (type_coreXmemoryXdisk Eg: B1_2X4X100)
instance_cpu | string | The amount of CPUs assigned to the instance. Defaults to 1
instance_memory | string | The amount of Memory (in bytes) assigned to the instance. Defaults to 1024
instance_network_speed | string | The network uplink speed, in megabits per second, which will be assigned to the instance. Defaults to 10
instance_disk_capacity | string | The amount of Disk capacity (in gigabytes) assigned to the instance. Defaults to 25. **Note:** Either use `instance_flavor` or `instance_cpu`, `instance_memory`, `instance_network_speed` 
communicator | string | To opt between SSH (for Linux) and winrm (for Windows)
base_os_code | string | If you would like to start from a pre-installed SoftLayer OS image, you can specify it's reference code. **Note:** you can use only one of `base_image_id` or `base_os_code` per builder configuration. To view all of the currently available pre-installed os images, run: `$ curl https://<username>:<api_key>@api.softlayer.com/rest/v3/SoftLayer_Virtual_Guest/getCreateObjectOptions.json | grep operatingSystemReferenceCode`
upload_to_datacenters | int | Datacenter ID to which Image has to be uploaded to. Multiple DCs supported seperated by ','
datacenter_name | string | The code name of the region to launch the instance in. Consequently, this is the region where the image will be available. This defaults to "ams01"
image_description | string | The description text which will be available for the resulting image. Defaults to "Instance snapshot. Generated by packer.io"
image_type | string | The type of the image to create. Only "standard" is supported
instance_domain | string | The domain assigned to the instance. Defaults to "provisioning.com"
ssh_port | string | The port that SSH will be available on. Defaults to port 22
ssh_timeout | string | The time to wait for SSH to become available before timing out. The format of this value is a duration such as "5s" or "5m". The default SSH timeout is "1m". Defaults to "15m"
ssh_private_key_file | string | Use this ssh private key file instead of a generated ssh key pair for connecting to the instance.
instance_state_timeout | string | The time to wait, as a duration string, for an instance or image snapshot to enter a desired state (such as "active") before timing out. The default state timeout is "25m"
ssh_host_key_file | | The SSH key that will be used to run the SSH server on the host machine to forward commands to the target machine
ssh_authorized_key_file | | The SSH public key of the Ansible ssh_user. 


As already stated above, a good way of reviewing the available options is by inspecting the output of the following API call: `curl -X GET --user <username>:<api_key> "https://api.softlayer.com/rest/v3/SoftLayer_Virtual_Guest/getCreateObjectOptions.json"`
