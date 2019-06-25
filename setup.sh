#!/bin/bash
#
# Softlayer packer builder deployment script -
#
# 1) Verify GO installed. If workspace does not exist, create one
# 2) Verify Packer is installed
# 3) Verify git installed (User needs to ensure ssh keys setup correctly)
# 3) Clone the Softlayer packer builder project
# 4) Install dependencies

echo "+============================================================+"
echo "|                   Pre Requisites                           |"
echo "| Make sure you have GO installed on this machine            |"
echo "| Make sure you have Packer installed on this machine        |"
echo "| Make sure you have git installed on this machine and have |
  | ssh keys configured to clone from github.ibm.com               |"
echo "+============================================================+"

sleep 4

# --Verify go installed--
go version >/dev/null 2>&1
if [ $? -eq 0 ]; then
    echo "GO exists"
else
    echo "Go is not installed on this machine, aborting..."
    exit 1
fi

# Validate GOPATH and GOROOT set

GOENV_GOPATH=$(go env | grep GOPATH)
GOENV_GOROOT=$(go env | grep GOPATH)
GOPATH=$(cut -d'"' -f2 <<<"$GOENV_GOPATH")
GOROOT=$(cut -d'"' -f2 <<<"$GOENV_GOROOT")
echo $GOPATH
echo $GOROOT

# --Validate go workspace exists--

ENV_HOME=$(env | grep HOME)
HOME=$(cut -d'=' -f2 <<<"$ENV_HOME")
echo $HOME

cd $HOME/go/src/github.com >/dev/null 2>&1
if [ $? -eq 0 ]; then
  echo "[INFO] Workspace exists"
else
  echo "[WARNING] Workspace does not exist, creating one..."
  mkdir $HOME/go
  mkdir $HOME/go/src
  mkdir $HOME/go/src/github.com
  mkdir $HOME/go/bin

  cd $HOME/go/src/github.com >/dev/null 2>&1
  if [ $? -eq 0 ]; then
    echo "[INFO] Workspace created successfully"
  else
    echo "[ERROR] Error creating go workspace, aborting..."
    exit 1
  fi
fi

# --Validate Packer--
# If OS == Centos, create a new syslink to packer.io to avoid
# conflict with the "packer" application that comes installed by default with Centos

# Linux v MAC
OS=$(uname)
if [[ "$OS" == "Linux" ]]; then
  echo "[INFO] OS: Linux"
  DIST=$(cat /etc/redhat-release | awk '{print $1}')
  echo $DIST
fi

if [ "$DIST" == "CentOS" ]; then
    echo "[INFO] Creating symbolic link for packer.io"
    #sudo ln -s /usr/local/packer /usr/local/bin/packer.io
    packer.io -v >/dev/null 2>&1
    if [ $? -eq 0 ];then
      echo "[INFO] Packer exists"
    else
      echo "[ERROR] Packer not installed on this machine, aborting..."
      exit 1
    fi
else
  # OS == MAC/Darwin
  packer -v >/dev/null 2>&1
  if [ $? -eq 0 ]; then
      PACKER_EXISTS=1
      echo "[INFO] Packer exists"
  else
      echo "[ERROR] Packer not installed on this machine, aborting..."
      exit 1
  fi
fi

# -- Validate git --

git --version >/dev/null 2>&1
if [ $? -eq 0 ];then
  echo "[INFO] Git exists"
else
  echo "[ERROR] git not found on machine, will install git now..."
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

# -- dependencies: get hashicorp --

go get github.com/hashicorp/packer >/dev/null 2>&1
if [ $? -eq 0 ];then
  echo "[INFO] Downloaded Hashicorp dependencies successfully"
else
  echo "[ERROR] Error downloading Hashicorp dependencies"
  exit 1
fi

# Remove vendor golang.org

cd $HOME/go/src/github.com/hashicorp/packer/vendor
rm -r golang.org >/dev/null 2>&1
if [ $? -eq 0 ];then
  echo "[INFO] Removed vendor golang.org directory successfully"
else
  echo "[ERROR] Error removing vendor golang.org directory"
fi

# --Create base directory and clone packer-softlayer-builder--

mkdir $HOME/go/src/github.com/softlayer
cd $HOME/go/src/github.com/softlayer
git clone git@github.ibm.com:StrategicOnboarding/packer-builder-softlayer.git

# --golang dependencies--

cd $HOME/go/src
git clone git@github.ibm.com:StrategicOnboarding/golang.org.git
go get -u golang.org/x/crypto/...
go get -u golang.org/x/sys
go get -u golang.org/x/tools

echo "+============================================================+"
echo "|               Completed successfully!                      |"
echo "+============================================================+"
