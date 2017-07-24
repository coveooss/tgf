FROM alpine:latest

ARG TERRAFORM_VERSION=0.9.11
ARG TERRAGRUNT_VERSION=0.12.24.08
ARG GOTEMPLATE_VERSION=1.01

ARG EXE_FOLDER=/usr/local/bin

LABEL vendor="Coveo"
LABEL maintainer "jgiroux@coveo.com"

RUN apk update && apk add openssl ca-certificates libc6-compat

RUN wget https://releases.hashicorp.com/terraform/${TERRAFORM_VERSION}/terraform_${TERRAFORM_VERSION}_linux_amd64.zip -O terraform.zip && unzip terraform.zip && mv terraform ${EXE_FOLDER} && rm terraform.zip
RUN wget https://github.com/coveo/terragrunt/releases/download/v${TERRAGRUNT_VERSION}/terragrunt_${TERRAGRUNT_VERSION}_linux_64-bits.zip -O terragrunt.zip && unzip terragrunt.zip && mv terragrunt ${EXE_FOLDER} && chmod +x ${EXE_FOLDER}/terragrunt && rm terragrunt.zip
RUN wget https://github.com/coveo/gotemplate/releases/download/v${GOTEMPLATE_VERSION}/gotemplate_linux -O ${EXE_FOLDER}/gotemplate && chmod +x ${EXE_FOLDER}/gotemplate
