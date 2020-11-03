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
RUN echo "[Step 1-1]: Install go and Create Workspace"
###########################################################
ENV GO_VERSION 1.14.7
ENV GO_TAR go$GO_VERSION.linux-amd64.tar.gz
ENV GO_URL https://golang.org/dl/$GO_TAR  
RUN set -ex \ 
    && curl -OL $GO_URL \
    && tar -C /usr/local -xzf $GO_TAR \
    && mkdir -p $HOME/go/src/github.com \
    && rm -rf $GO_TAR


###########################################################
RUN echo "[Step 1-2]: Set go Environment variables"
###########################################################
ENV GOPATH $HOME/go
ENV GOROOT /usr/local/go
ENV PATH $PATH:$GOPATH/bin:$GOROOT/bin
RUN set -ex \
    && cd $HOME \
    && echo export GOROOT=/usr/local/go >> .profile \
    && echo export GOPATH=$HOME/go >> .profile


###########################################################
RUN echo "[Step 2]: Setup Ansible"
###########################################################
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -y install ansible

# Fix "winrm or requests is not installed: No module named winrm"
RUN apt -y install python3-pip
RUN pip3 install --ignore-installed "pywinrm>=0.2.2"    


###########################################################
RUN echo "[Step 3-1]: Download and install Packer"
###########################################################
ENV PACKER_VERSION 1.6.1
ENV PACKER_ZIP packer_"$PACKER_VERSION"_linux_amd64.zip
ENV PACKER_URL https://releases.hashicorp.com/packer/1.6.1/$PACKER_ZIP
RUN set -ex \
    && cd /temp \
    && curl -OL $PACKER_URL \
    && mkdir -p /usr/local/packer \
    && unzip $PACKER_ZIP -d /usr/local/packer \
    && rm -rf $PACKER_ZIP


###########################################################
RUN echo "[Step 3-2]: Set packer Environment variables"
###########################################################
ENV PACKERPATH /usr/local/packer
ENV PATH $PATH:$PACKERPATH
RUN set -ex \
    && cd $HOME \
    && echo export PACKERPATH=/usr/local/packer >> .profile


###########################################################
RUN echo "[Step 4]: Download Packer dependencies"
###########################################################
RUN set -ex \
    && cd $GOPATH/src/github.com \
    && go get github.com/hashicorp/packer \
    && go get golang.org/x/text


###########################################################
RUN echo "[Step 5]: Install HCL2 dependencies"
###########################################################
RUN set -ex \    
    && go get github.com/cweill/gotests/... \
    && go install github.com/hashicorp/packer/cmd/mapstructure-to-hcl2 \
    && mv $GOPATH/src/github.com/hashicorp/packer/vendor/github.com/hashicorp/hcl $GOPATH/src/github.com/hashicorp


###########################################################
RUN echo "[Step 7]: Access IBM Cloud Packer plugin"
###########################################################
# Copy source code to the folder packer-builder-ibmcloud
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