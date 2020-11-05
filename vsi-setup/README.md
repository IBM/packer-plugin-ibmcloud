# IBM Packer Plugin Linux-based VSI Setup

### Install it using a shell script  
1. Go to vsi-setup folder  
   `$ cd vsi-setup`
2. Choose the right installation for your instance: macOS, ubuntu. 
   - Here, setup-ubuntu.sh is used.
3. Copy file to a folder called "temp" inside linux machine
4. Give permission to setup-ubuntu.sh (Setup plugin on your machine)
   - `chmod +x setup-ubuntu.sh`
5. run `. ./setup-ubuntu.sh`
6. run `go generate ./builder/ibmcloud/...`
7. run `go build`
8. Follow steps 4-6 Using a Docker Container