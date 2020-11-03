# IBM Packer Plugin Linux-based VSI Setup

### Install it using a shell script  
1. Copy files to a folder called "temp" inside linux machine
2. Give permission to setup.sh (Setup plugin on your machine)
   - `chmod +x setup.sh`
3. run `. ./setup.sh`
4. run `go generate ./builder/ibmcloud/...`
5. run `go build`