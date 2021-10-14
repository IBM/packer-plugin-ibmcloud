# Base image
FROM ubuntu:latest
LABEL maintainer = "Juan.Pinzon@ibm.com"

ENV GO_VERSION 1.17.2
ENV PACKER_VERSION 1.7.6

ARG GO_VERSION
ARG PACKER_VERSION
ENV GO_VERSION ${GO_VERSION}
ENV PACKER_VERSION ${PACKER_VERSION}

ENV HOME /root

RUN set -ex \ 
  && apt-get -y update \
  && apt-get -y install apt-utils curl git unzip vim \
  && mkdir -p /packer-plugin-ibmcloud

# Set the working directory
WORKDIR /packer-plugin-ibmcloud

###########################################################
RUN echo "[Step 1]: Install go and set go Environment variables"
###########################################################
ENV GO_TAR go${GO_VERSION}.linux-amd64.tar.gz
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
ENV DEBIAN_FRONTEND=noninteractive
RUN apt-get -y install ansible

# Fix "winrm or requests is not installed: No module named winrm"
RUN apt -y install python3-pip
RUN pip3 install --ignore-installed "pywinrm>=0.2.2"
RUN echo "Ansible Installation Successfully Completed."

###########################################################
RUN echo "[Step 3]: Install Packer and set Packer's Environment variables"
###########################################################
ENV PACKER_ZIP packer_${PACKER_VERSION}_linux_amd64.zip
ENV PACKER_URL https://releases.hashicorp.com/packer/$PACKER_VERSION/$PACKER_ZIP
RUN set -ex \
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
RUN echo "[Step 4]: Build IBM Cloud Packer Plugin binary"
###########################################################
COPY . ./
RUN set -ex \
  && cd ./ \
  && go install github.com/hashicorp/packer-plugin-sdk/cmd/packer-sdc@latest \
  && go mod tidy \
  && go mod vendor \  
  && go generate ./builder/ibmcloud/... \
  && go mod vendor \
  && go build .
RUN echo "IBM Cloud Packer Plugin binary Successfully Created."

# Comment below line to make container interactive
ENTRYPOINT ["/usr/local/packer/packer"]
