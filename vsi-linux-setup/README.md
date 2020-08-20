# IBM Packer Plugin Linux-based VSI Setup

### Using a Docker Container  
1. Build the script from Dockerfile  
    `$ cd vsi-linux-setup`  
    `$ docker build -t ibmcloudvpc/packer-plugin-ibmcloud .`   
   OR  
   Pull the image from ibmcloudvpc/packer-plugin-ibmcloud  
    `$ docker pull ibmcloudvpc/packer-plugin-ibmcloud:latest`  
2. Check image is in the local Docker image registry  
    `$ docker image ls`
3. Run and interact with the container  
    `$ docker run -it ibmcloudvpc/packer-plugin-ibmcloud /bin/bash`    
4. Copy/Create SSH Keys on /root/.ssh folder   
    - To create them run `$ ssh-keygen -t rsa` 
5. Update .env file with your IBM Cloud credentials  
    `$ cd $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud`  
    `$ vi .env`      
6. Run Packer plugin commands  
    `$ source .env`  
    `$ packer validate examples/linux.json`  
    `$ packer build examples/linux.json`


### Install it using a shell script  
1. Go to vsi-linux-setup folder  
   `$ cd vsi-linux-setup`
2. Copy files to a folder called "temp" inside linux machine
3. Give permission to setup.sh (Setup plugin on your machine)
   1. `chmod +x setup.sh`
4. run `. ./setup.sh`
5. run `go generate ./builder/ibmcloud/...`
6. run `go build`