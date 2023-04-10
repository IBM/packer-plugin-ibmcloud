# IBM Packer Plugin
The IBM Packer Plugin can be used to create custom Images on IBM Cloud. There is one Packer Builder for Classic Infrastructure and one Packer Builder for VPC Infrastructure.

## Description
The builder takes a source OS base Linux or Windows image (identified by it's global ID), provisions an Instance, adds additional applications and services to it and generates an Image out of the Instance. This generated Image can be reused to launch new VSI Instances within IBM Cloud.
The builder does not manage Images. Once it creates an Image, it is up to you to use it or delete it.

### Builders
- [classic](builders/classic) - The `classic` builder support the creation of Image template(.VHD) with pre-configured OS and installed softwares on IBM Cloud - Classic Infrastructure. **- Not yet available. Clone `classic` branch instead**
- [vpc](builders/vpc) - The `vpc` builder support the creation of custom Images on IBM Cloud - VPC Infrastructure.


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

   Also, create a file `variables.pkrvars.hcl` with the following content.
   ```
   SUBNET_ID = ""
   REGION = ""
   SECURITY_GROUP_ID = ""
   RESOURCE_GROUP_ID = ""
   IBM_API_KEY = ""
   ```

- Customize your Packer Template: see [Configuration](#configuration) to find a detail description of each field on the Template. Likewise, there are some Packer Template examples on `examples` folder.
- Create container with Packer Plugin Binary within it:
  run `make image`

#### 2. Run Packer
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
