# packer-linux-setup

1. copy files to a folder called "temp" inside linux machine
2. give permission to setup.sh and clean.sh
   1. chmod +x setup.sh
   2. chmod +x clean.sh
3. run . ./clean.sh
4. run . ./setup.sh
5. run go generate ./builder/ibmcloud/...
6. run go build