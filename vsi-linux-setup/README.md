# IBM Packer Plugin Linux-based VSI Setup

### Using a sheel script
1. copy files to a folder called "temp" inside linux machine
2. give permission to setup.sh
   1. chmod +x setup.sh
3. run . ./setup.sh
4. run go generate ./builder/ibmcloud/...
5. run go build

### Using the Docker Container
1. Build the script from Dockerfile  
    $ cd vsi-linux-setup
    $ docker build -t ibmcloudvpc/packer-plugin-ibmcloud .  
   OR  
   Pull the image from ibmcloudvpc/packer-plugin-ibmcloud  
    $ docker pull ibmcloudvpc/packer-plugin-ibmcloud:latest  
2. Check image is in the local Docker image registry  
    $ docker image ls
3. Run and interact with the container  
    $ docker run -it ibmcloudvpc/packer-plugin-ibmcloud /bin/bash  
4. Copy/Create SSH Keys on /root/.ssh folder   
    - To create them run $ ssh-keygen -t rsa 
5. Update .env file with your IBM Cloud credentials  
    $ cd $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud 
    $ vi .env    
6. Run Packer plugin commands  
    $ source .env  
    $ packer validate examples/linux.json  
    $ packer build examples/linux.json