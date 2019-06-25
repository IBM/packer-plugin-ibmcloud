#!/bin/bash
echo "+============================================================+"
echo "|                   Pre Requisites                           |"
echo "| Make sure you have GO installed on this machine            |"
echo "| Make sure you have Packer installed on this machine        |"
echo "| Make sure you have git installed on this machine and have  |"
echo "| ssh keys configured to clone from github.ibm.com           |"
echo "+============================================================+"
source credentials      # import credentials
source stuff            # import stuff

sleep 4
# Create ibmcloud Directory if not Exists
[ ! -d $HOME/ibmcloud ] && mkdir -p $HOME/ibmcloud
cd $HOME/ibmcloud > /dev/null


echo "$cyan [Step 1-1]: Download and install go $white"
go version > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "GO exists $white"
else
    echo "Installing go... $white"
    OS=$(uname)
    if [[ "$OS" == "Linux" ]]; then
      echo "OS: Linux"
      sudo apt-get update
      sudo apt-get upgrade
      wget $go_url > /dev/null
      tar -xvf $go_tar > /dev/null
      mv go /usr/local > /dev/null
    fi
fi
echo "$green [INFO] go installed successfully $white"


echo "$cyan [Step 1-2]: Validate go workspace exist $white"
ENV_HOME=$(env | grep HOME)
HOME=$(cut -d'=' -f2 <<<"$ENV_HOME")

cd $HOME/go/src/github.com > /dev/null 2>&1
if [ $? -eq 0 ]; then
  echo "$green [INFO] Workspace exists $white"
else
  echo "$yellow [WARNING] Workspace does not exist, creating one... $white"
  mkdir $HOME/go > /dev/null
  mkdir $HOME/go/src > /dev/null
  mkdir $HOME/go/src/github.com > /dev/null
  mkdir $HOME/go/bin > /dev/null

  cd $HOME/go/src/github.com > /dev/null 2>&1
  if [ $? -eq 0 ]; then
    echo "$green [INFO] Workspace created successfully $white"
  else
    echo "$red [ERROR] Error creating go workspace, aborting... $white"
    exit 1
  fi
fi


echo "$cyan [Step 1-3]: Set go Environment variables $white"
export GOROOT=/usr/local/go                         # Where go is installed
export GOPATH=$HOME/go                              # Your Go Workspace - your app location
export PATH=$PATH:$GOPATH/bin:$GOROOT/bin
cd $HOME
echo export GOROOT=/usr/local/go >> .profile
echo export GOPATH=$HOME/go >> .profile
echo export PATH=$PATH:$GOPATH/bin:$GOROOT/bin >> .profile
echo "$green [INFO] Successfully set of go Environment variables $white"


echo "$cyan [Step 2-1]: Validate Packer $white"
OS=$(uname)
if [ "$DIST" == "CentOS" ]; then
    echo "$green [INFO] Creating symbolic link for packer.io $white"
    #sudo ln -s /usr/local/packer /usr/local/bin/packer.io
    packer.io -v > /dev/null 2>&1
    if [ $? -eq 0 ];then
      echo "$green [INFO] Packer exists $white"
    else
      echo "$red [ERROR] Packer not installed on this machine, aborting... $white"
      exit 1
    fi
else
  # OS == MAC/Darwin
  packer -v > /dev/null 2>&1
  if [ $? -eq 0 ]; then
      PACKER_EXISTS=1
      echo "$green [INFO] Packer exists $white"
  else
      echo "$yellow [WARNING] ./clean Packer not installed on this machine, Installing packer... $white"
      cd $HOME/ibmcloud
      sudo apt-get install unzip 
      wget $packer_url > /dev/null
      unzip $packer_zip -d packer > /dev/null
      mv packer /usr/local/ > /dev/null
  fi
fi


echo "$cyan [Step 2-2]: Set packer Environment variables $white"
export PACKERPATH=/usr/local/packer
export PATH=$PATH:$PACKERPATH
cd $HOME
echo export PACKERPATH=/usr/local/packer >> .profile
echo export PATH=$PATH:$PACKERPATH >> .profile
cd $HOME/ibmcloud
echo "$green [INFO] Successfully set of packer Environment variables $white"


echo "$cyan [Step 3]: Validate git $white"
git --version > /dev/null 2>&1
if [ $? -eq 0 ];then
  echo "$green [INFO] Success - Git exists $white"
else
  echo "$red [ERROR] git not found on machine, will install git now... $white"
  # <--TODO: GIT needs to be installed and "CONFIGURED"
  # since users need to have their private key setup to allow
  # cloning from ibm github

  #yes | sudo yum install git
  #git --version >/dev/null 2>&1
  #if [ $? -eq 0 ]; then
  #    echo "[INFO] git installed successfully"
  #else
  #  echo "[ERROR] Error instaling git on this machine, aborting"
  exit 1
  #fi
fi


echo "$cyan [Step 4]: Setup ssh keys $white"
#Copy keys into ..ssh folder
if [ -d $HOME/.ssh ] 
then
    echo "$yellow [WARNING] old .ssh directory saved as .ssh-old $white"
    mv $HOME/.ssh $HOME/.shh-old > /dev/null
else
    echo "$yellow [WARNING] .ssh directory not found, creating it now... $white"
    mkdir $HOME/.ssh > /dev/null
fi

cd $HOME/.ssh/ > /dev/null
echo "$id_rsa" > id_rsa
echo "$id_rsa_pub" > id_rsa.pub
chmod 600 $HOME/.ssh/id_rsa.pub > /dev/null
chmod 600 $HOME/.ssh/id_rsa > /dev/null
echo "$green [INFO] Done ssh keys $white"


echo "$cyan [Step 5-1]: Download Packer dependencies $white"
# -- dependencies: get hashicorp --
go get github.com/hashicorp/packer > /dev/null 2>&1
if [ $? -eq 0 ];then
  echo "$green [INFO] Downloaded Hashicorp dependencies successfully $white"
else
  echo "$red [ERROR] Error downloading Hashicorp dependencies $white"
  exit 1
fi


echo "$cyan [Step 5-2]: Remove vendor golang.org $white"
cd $HOME/go/src/github.com/hashicorp/packer/vendor > /dev/null
rm -r golang.org > /dev/null 2>&1
if [ $? -eq 0 ];then
  echo "$green [INFO] Removed vendor golang.org directory successfully $white"
else
  echo "$red [ERROR] Error removing vendor golang.org directory $white"
fi


echo "$cyan [Step 6]: SoftLayer Packer-Builder $white"
echo "$cyn Create base directory and clone packer-softlayer-builder $white"
mkdir $HOME/go/src/github.com/softlayer > /dev/null
cd $HOME/go/src/github.com/softlayer > /dev/null
git clone -b add-winrm  git@github.ibm.com:GCAT/packer-builder-softlayer.git > /dev/null
echo "$green [INFO] Done SoftLayer Packer-Builder $white"


#echo "$cyan [Step 7]: Setup the golang.org directory $white"
#cd $GOPATH/src > /dev/null
#mv golang.org golang.org-old > /dev/null    #create copy old golang.org
#git clone git@github.ibm.com:GCAT/golang.org.git > /dev/null
#echo "$green [INFO]: Done setup the golang.org directory $white"

echo "$cyan [Step 7]: Setup the golang.org directory $white"
cd $GOPATH/src > /dev/null
mv golang.org golang.org-old > /dev/null    #create copy old golang.org
git clone git@github.ibm.com:GCAT/golang.org.git > /dev/null
cd $GOPATH/src/golang.org/x/ > /dev/null
rm -rf * > /dev/null
# clone repos as dependency used to build plugin
git clone https://github.com/golang/crypto.git > /dev/null
git clone https://github.com/golang/oauth2.git > /dev/null
git clone https://github.com/golang/net.git > /dev/null
git clone https://github.com/golang/sys.git > /dev/null
git clone https://github.com/golang/time.git > /dev/null
git clone https://github.com/golang/text.git > /dev/null
cd $GOPATH/src > /dev/null
go get -u cloud.google.com/go/compute/metadata > /dev/null
echo "$green [INFO]: Done setup the golang.org directory $white"

echo "$cyan [Step 8]:  Access ibmcloud packer $white"
cd $HOME/go/src/github.com/softlayer/packer-builder-softlayer > /dev/null
go build > /dev/null
echo "$green [INFO]: success doing $ go build $white"

echo "$cyan [Step 10]: packer validate ....$white"
#echo "$windows_test" > examples/windows.json
packer validate examples/windows.json > /dev/null
echo "$green [INFO]: Packer successfully validated windows.json $white"

echo "Go to $HOME/go/src/github.com/softlayer/packer-builder-softlayer"
echo "+============================================================+"
echo "|               Completed successfully!                      |"
echo "+============================================================+"
