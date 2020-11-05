#!/bin/bash
# run it as . ./setup.sh  === source ./seup.sh    -> Setup ENV variables
echo "+===========================================================+"
echo "|                   IBM Packer Plugin                       |"
echo "+===========================================================+"

cd $HOME  # To Set Up .profile file with Environment variables
###########################################################
echo "[Step 1]: Install go and set go Environment variables"
###########################################################
go version > /dev/null 2>&1
if [ $? -eq 0 ]; then
  echo "GO already installed."
else
  echo "Installing go..."
  GO_VERSION=1.15.3
  GO_TAR=go$GO_VERSION.linux-amd64.tar.gz
  GO_URL=https://golang.org/dl/$GO_TAR
  curl -OL $GO_URL > /dev/null
  tar -C /usr/local -xzf $GO_TAR > /dev/null
  mkdir -p $HOME/go/src/github.com > /dev/null
  rm $GO_TAR > /dev/null

  echo "Setting go Environment variables..."
  GOPATH=$HOME/go
  GOROOT=/usr/local/go
  PATH=$PATH:$GOPATH/bin:$GOROOT/bin
  echo export GOPATH=$GOPATH >> .profile
  echo export GOROOT=$GOROOT >> .profile
  echo export PATH=$PATH >> .profile
  echo "go Installation Successfully Completed."
fi


###########################################################
echo "[Step 2]: Setup Ansible"
###########################################################
ansible --version > /dev/null 2>&1
if [ $? -eq 0 ]; then
  echo "Ansible already installed."
else
  echo "Installing Ansible..."
  export DEBIAN_FRONTEND=noninteractive
  apt update > /dev/null
  apt --yes install software-properties-common > /dev/null
  apt-add-repository --yes --update ppa:ansible/ansible > /dev/null
  apt --yes install ansible > /dev/null

  # Fix "winrm or requests is not installed: No module named winrm"
  apt -y install python3-pip > /dev/null
  pip3 install --ignore-installed "pywinrm>=0.2.2" > /dev/null
  rm get-pip.py > /dev/null
  echo "Ansible Installation Successfully Completed."
fi


###########################################################
echo "[Step 3]: Install Packer and set Packer's Environment variables"
###########################################################
packer --version > /dev/null 2>&1
if [ $? -eq 0 ]; then
  echo "Packer already installed."
else
  echo "Installing Packer..."
  apt --yes install unzip > /dev/null
  PACKER_VERSION=1.6.5 
  PACKER_ZIP=packer_"$PACKER_VERSION"_darwin_amd64.zip
  PACKER_URL=https://releases.hashicorp.com/packer/1.6.5/$PACKER_ZIP
  curl -OL $PACKER_URL > /dev/null
  mkdir -p /usr/local/packer > /dev/null
  unzip $PACKER_ZIP -d /usr/local/packer > /dev/null
  rm $PACKER_ZIP > /dev/null

  echo "Setting Packer Environment variables..."
  PACKERPATH=/usr/local/packer
  PATH=$PATH:$PACKERPATH
  echo export PATH=$PATH >> .profile
  echo "Packer Installation Successfully Completed."
fi



###########################################################
echo "[Step 4]: Download Packer dependencies"
###########################################################
# See go.mod for other dependencies
echo "Installing Packer dependencies..."
cd $GOPATH/src/github.com
go get github.com/hashicorp/packer > /dev/null
go get golang.org/x/text > /dev/null

echo "Installing HCL2 dependencies"
go get github.com/cweill/gotests/... > /dev/null
go install github.com/hashicorp/packer/cmd/mapstructure-to-hcl2 > /dev/null
mv $GOPATH/src/github.com/hashicorp/packer/vendor/github.com/hashicorp/hcl $GOPATH/src/github.com/hashicorp
echo "Packer Dependencies Installation Successfully Completed."


###########################################################
echo "[Step 5]: Access IBM Cloud Packer plugin"
###########################################################
# Copy source code to the folder packer-builder-ibmcloud
mkdir -p $GOPATH/src/github.com/ibmcloud > /dev/null
cd $GOPATH/src/github.com/ibmcloud
git clone https://github.com/IBM/packer-plugin-ibmcloud.git packer-builder-ibmcloud > /dev/null
cd $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud 
go generate ./builder/ibmcloud/...
go build

###########################################################
echo "IBM Packer Plugin created successfully!!!"
###########################################################