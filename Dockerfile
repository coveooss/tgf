FROM alpine:latest

ARG TERRAFORM_VERSION=0.9.11
ARG TERRAFORM_QUANTUM=0.2
ARG TERRAGRUNT_VERSION=0.12.24.01
ARG GOTEMPLATE_VERSION=1.01
ARG TFLINT_VERSION=0.3.6

LABEL vendor="Coveo"
LABEL maintainer "jgiroux@coveo.com"

RUN apk update && apk add openssl ca-certificates

RUN wget https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip -O terraform.zip && unzip terraform.zip && mv terraform /usr/local/bin && rm terraform.zip
RUN wget https://github.com/coveo/terraform-provider-quantum/releases/download/v${TERRAFORM_QUANTUM}/terraform-provider-quantum_linux_x64 -O /usr/local/bin/terraform-provider-quantum && chmod +x /usr/local/bin/terraform-provider-quantum
RUN wget https://github.com/coveo/terragrunt/releases/download/v${TERRAGRUNT_VERSION}/terragrunt_linux_amd64 -O /usr/bin/terragrunt && chmod +x /usr/bin/terragrunt
RUN wget https://github.com/coveo/gotemplate/releases/download/v${GOTEMPLATE_VERSION}/gotemplate_linux -O /usr/bin/gotemplate && chmod +x /usr/bin/gotemplate
RUN wget https://github.com/wata727/tflint/releases/download/v${TFLINT_VERSION}/tflint_linux_amd64.zip -O tflint.zip && unzip tflint.zip && mv tflint /usr/local/bin && rm tflint.zip

RUN apk add bash zsh fish
RUN apk add py2-pip python3
RUN pip install --upgrade pip
RUN pip install awscli

# Install JQ
RUN apk add jq

# Install terraforming
RUN apk add ruby ruby-rdoc ruby-irb
RUN gem install terraforming
