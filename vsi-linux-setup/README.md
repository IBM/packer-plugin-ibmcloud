# IBM Packer Plugin Linux-based VSI Setup

### Using a sheel script
1. copy files to a folder called "temp" inside linux machine
2. give permission to setup.sh and clean.sh
   1. chmod +x setup.sh
   2. chmod +x clean.sh
3. run . ./clean.sh
4. run . ./setup.sh
5. run go generate ./builder/ibmcloud/...
6. run go build

### Using a Docker Container
1. Build the script from Dockerfile  
    $ docker build -t packer-builder-ibmcloud .   
2. Check image is in the local Docker image registry  
    $ docker image ls
3. Start and interact with the container  
    $ docker run -it packer-builder-ibmcloud /bin/bash  
4. Copy/Create SSH Keys on /root/.ssh folder   
    - To create them run $ ssh-keygen -t rsa 
5. Update .env file with your IBM Cloud credentials  
    $ cd $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud 
    $ vi .env    
6. Run Packer plugin commands  
    $ source .env  
    $ packer validate examples/linux.json  
    $ packer build examples/linux.json