#!/bin/bash
# run it as . ./setup.sh  === source ./seup.sh    -> Setup ENV variables
echo "+===========================================================+"
echo "|                   IBM Packer Plugin                       |"
echo "| [Step 1]: Setup Go                                        |"
echo "| [Step 1-1]: Install go and Create Workspace               |"
echo "| [Step 1-2]: Set go Environment variables                  |"
echo "| [Step 2]: Setup Packer                                    |"
echo "| [Step 2-1]: Download and install Packer                   |"
echo "| [Step 2-2]: Set packer Environment variables              |"
echo "| [Step 3]: Setup git                                       |"
echo "| [Step 4]: Setup Packer                                    |"
echo "| [Step 4-1]: Download Packer dependencies                  |"
echo "| [Step 4-2]: Remove vendor golang.org                      |"
echo "| [Step 5]: Setup the golang.org directory                  |"
echo "| [Step 6]: Setup Ansible                                   |"
echo "| [Step 7]: Access IBM Cloud Packer plugin                  |"
echo "| [Step 8]: packer validate ....                            |"
echo "| Make sure you have Packer installed on this machine       |"
echo "| Make sure you have git installed on this machine          |"
echo "+===========================================================+"
source stuff            # import stuff

sleep 4
echo "$cyan [Step 1-1]: Install go and Create Workspace $white"
go version > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "GO exists $white"
else
    echo "Installing go... $white"
    OS=$(uname)
    if [[ "$OS" == "Linux" ]]; then
      echo "OS: Linux"
      # sudo apt-get update
      # yes | sudo apt-get upgrade
      curl -O $go_url > /dev/null
      tar -C /usr/local -xzf $go_tar > /dev/null
      mkdir -p $HOME/go/src/github.com > /dev/null
    fi
fi
echo "$green [INFO] go installed and Create Workspace successfully$white"


echo "$cyan [Step 1-2]: Set go Environment variables $white"
GOPATH="$HOME/go"
GOROOT="/usr/local/go"
export PATH=$PATH:$GOPATH:$GOROOT/bin:$GOPATH/bin
cd $HOME
echo export GOROOT=/usr/local/go >> .profile
echo export GOPATH=$HOME/go >> .profile
echo "$green [INFO] Successfully set of go Environment variables and Created Workspace$white"


echo "$cyan [Step 2-1]: Download and install Packer $white"
packer -v > /dev/null 2>&1
if [ $? -eq 0 ]; then
  PACKER_EXISTS=1
  echo "$green [INFO] Packer exists $white"
else
  echo "$yellow [WARNING] ./clean Packer not installed on this machine, Installing packer... $white"
  cd $HOME/temp
  yes | sudo apt-get install unzip
  curl -O $packer_url > /dev/null
  mkdir -p /usr/local/packer > /dev/null
  unzip $packer_zip -d /usr/local/packer > /dev/null
fi


echo "$cyan [Step 2-2]: Set packer Environment variables $white"
PACKERPATH="/usr/local/packer"
export PATH=$PATH:$PACKERPATH
cd $HOME
echo export PACKERPATH=/usr/local/packer >> .profile
echo export PATH=$PATH:$GOPATH:$GOROOT/bin:$GOPATH/bin:$PACKERPATH >> .profile
echo "$green [INFO] Successfully set of packer Environment variables $white"


echo "$cyan [Step 3]: Validate git $white"
git --version > /dev/null 2>&1
if [ $? -eq 0 ];then
  echo "$green [INFO] Success - Git exists $white"
else
  echo "$red [ERROR] git not found on machine, will install git now... $white"
  yes | sudo apt-get install git
  git --version >/dev/null 2>&1
  if [ $? -eq 0 ]; then
    echo "[INFO] git installed successfully"
  else
    echo "[ERROR] Error instaling git on this machine, aborting"
    exit 1
  fi
fi


echo "$cyan [Step 4-1]: Download Packer dependencies $white"
# -- dependencies: get hashicorp --
cd $GOPATH/src/github.com > /dev/null
go get github.com/hashicorp/packer > /dev/null 2>&1
if [ $? -eq 0 ];then
  echo "$green [INFO] Downloaded Hashicorp dependencies successfully $white"
else
  echo "$red [ERROR] Error downloading Hashicorp dependencies $white"
  exit 1
fi


echo "$cyan [Step 4-2]: Remove vendor golang.org $white"
cd $GOPATH/src/github.com/hashicorp/packer/vendor > /dev/null
rm -r golang.org > /dev/null 2>&1
if [ $? -eq 0 ];then
  echo "$green [INFO] Removed vendor golang.org directory successfully $white"
else
  echo "$red [ERROR] Error removing vendor golang.org directory $white"
fi


echo "$cyan [Step 5]: Setup the golang.org directory $white"
mkdir -p $GOPATH/src/golang.org/x/ > /dev/null
cd $GOPATH/src/golang.org/x/ > /dev/null
# clone repos as dependency used to build plugin
git clone https://go.googlesource.com/crypto > /dev/null
git clone https://github.com/golang/oauth2.git > /dev/null
git clone https://go.googlesource.com/net > /dev/null
git clone https://go.googlesource.com/sys > /dev/null
git clone https://go.googlesource.com/time > /dev/null
git clone https://go.googlesource.com/text > /dev/null

# below packages are required after change above packages source
go get github.com/agext/levenshtein > /dev/null
go get github.com/mitchellh/go-wordwrap > /dev/null
mv $GOPATH/src/github.com/hashicorp/packer/vendor/github.com/zclconf $GOPATH/src/github.com
go get github.com/apparentlymart/go-textseg/textseg > /dev/null
cd /root/go/src/github.com/apparentlymart/go-textseg > /dev/null
mkdir v12 > /dev/null
cp -r textseg v12 > /dev/null

cd $GOPATH/src > /dev/null
go get -u cloud.google.com/go/compute/metadata > /dev/null
echo "$green [INFO]: Done setup the golang.org directory $white"


echo "$cyan [Step 6-1]: Setup Ansible $white"
sudo apt update
sudo apt --yes install software-properties-common
sudo apt-add-repository --yes --update ppa:ansible/ansible
sudo apt --yes install ansible
# Fix "winrm or requests is not installed: No module named winrm"
sudo apt --yes install python-pip
pip install --ignore-installed "pywinrm>=0.2.2"
echo "$green [INFO]: Done setup Ansible $white"


echo "$cyan [Step 7]: Access IBM Cloud Packer plugin $white"
mkdir -p $GOPATH/src/github.com/ibmcloud > /dev/null
cd $GOPATH/src/github.com/ibmcloud > /dev/null
# main repo
# git clone https://github.com/IBM/packer-plugin-ibmcloud.git > /dev/null
# issue branch
git clone -b i-4-jp --single-branch https://github.com/IBM/packer-plugin-ibmcloud.git packer-builder-ibmcloud
cd $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud
# Install dependencies for Generate the HCL2 code of a plugin
go get github.com/cweill/gotests/... > /dev/null
go install github.com/hashicorp/packer/cmd/mapstructure-to-hcl2 > /dev/null
mv $GOPATH/src/github.com/hashicorp/packer/vendor/github.com/hashicorp/hcl $GOPATH/src/github.com/hashicorp > /dev/null
go generate ./builder/ibmcloud/...

go build
echo "$green [INFO]: success doing $ go build $white"


echo "$cyan [Step 8]: packer validate ....$white"
source .env
packer validate examples/linux.json > /dev/null
echo "$green [INFO]: Packer successfully validated json script $white"

echo "+============================================================+"
echo "|               Completed successfully!                      |"
echo "+============================================================+"