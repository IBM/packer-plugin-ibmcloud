# packer-linux-setup


1. copy files to a folder called "ibmcloud" inside linux machine
2. give permission to setup.sh and clean.sh
   1. chmod 755 setup.sh
   2. chmod 755 clean.sh
3. Add ssh credentials to "credentials" file 
4. add softlayer credentials to "stuff" file which has the windows.json content
5. run ./clean.sh
6. run ./setup.sh
7. run go build
8. run packer validate examples/windows.json
9. run packer build examples/windows.json