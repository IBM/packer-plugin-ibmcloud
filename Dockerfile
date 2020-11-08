# Base image
FROM ubuntu:latest
# Update Ubuntu and install required packages
RUN set -ex \ 
    && apt-get -y update \
    && apt-get -y install apt-utils curl git unzip vim

# Set Maintainer
LABEL maintainer = "Juan.Pinzon@ibm.com"

# Set the working directory to /temp
WORKDIR /temp

# Set ENV variables 
ENV HOME /root

###########################################################
RUN echo "[Step 1]: Install go and set go Environment variables"
###########################################################
RUN echo "Installing go..."
ENV GO_VERSION 1.15.3
ENV GO_TAR go$GO_VERSION.linux-amd64.tar.gz
ENV GO_URL https://golang.org/dl/$GO_TAR  
RUN set -ex \ 
    && curl -OL $GO_URL \
    && tar -C /usr/local -xzf $GO_TAR \
    && mkdir -p $HOME/go/src/github.com \
    && rm -rf $GO_TAR

RUN echo "Setting go Environment variables..."
ENV GOPATH $HOME/go
ENV GOROOT /usr/local/go
ENV PATH $PATH:$GOPATH/bin:$GOROOT/bin 
RUN set -ex \
    && cd $HOME \
    && echo export GOPATH=$GOPATH >> .profile \
    && echo export GOROOT=$GOROOT >> .profile \
    && echo export PATH=$PATH >> .profile
RUN echo "go Installation Successfully Completed."


###########################################################
RUN echo "[Step 2]: Setup Ansible"
###########################################################
RUN echo "Installing Ansible..."
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -y install ansible

# Fix "winrm or requests is not installed: No module named winrm"
RUN apt -y install python3-pip
RUN pip3 install --ignore-installed "pywinrm>=0.2.2"
RUN echo "Ansible Installation Successfully Completed."


###########################################################
RUN echo "[Step 3]: Install Packer and set Packer's Environment variables"
###########################################################
RUN echo "Installing Packer..."
ENV PACKER_VERSION 1.6.5
ENV PACKER_ZIP packer_"$PACKER_VERSION"_linux_amd64.zip
ENV PACKER_URL https://releases.hashicorp.com/packer/1.6.5/$PACKER_ZIP
RUN set -ex \
    && cd /temp \
    && curl -OL $PACKER_URL \
    && mkdir -p /usr/local/packer \
    && unzip $PACKER_ZIP -d /usr/local/packer \
    && rm -rf $PACKER_ZIP

RUN echo "Setting Packer Environment variables..."
ENV PACKERPATH /usr/local/packer
ENV PATH $PATH:$PACKERPATH
RUN set -ex \
    && cd $HOME \
    && echo export PATH=$PATH >> .profile
RUN echo "Packer Installation Successfully Completed."


###########################################################
RUN echo "[Step 4]: Download Packer dependencies"
###########################################################
# See go.mod for other dependencies
RUN echo "Installing Packer dependencies..."
RUN set -ex \
    && cd $GOPATH/src/github.com \
    && go get github.com/hashicorp/packer \
    && go get golang.org/x/text

RUN echo "Installing HCL2 dependencies"
RUN set -ex \    
    && go get github.com/cweill/gotests/... \
    && go install github.com/hashicorp/packer/cmd/mapstructure-to-hcl2 \
    && mv $GOPATH/src/github.com/hashicorp/packer/vendor/github.com/hashicorp/hcl $GOPATH/src/github.com/hashicorp
RUN echo "Packer Dependencies Installation Successfully Completed."


###########################################################
RUN echo "[Step 5]: Access IBM Cloud Packer plugin"
###########################################################
# Copy source code to the folder packer-builder-ibmcloudvpc
RUN set -ex \
    && mkdir -p $GOPATH/src/github.com/ibmcloud
COPY . $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud

RUN set -ex \
    && cd $GOPATH/src/github.com/ibmcloud/packer-builder-ibmcloud \
    && go generate ./builder/ibmcloud/... \
    && go build

###########################################################
RUN echo "IBM Packer Plugin created successfully!!!"
###########################################################