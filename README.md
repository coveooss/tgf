# TGF

A **T**erra**g**runt **f**rontend that allow execution of Terragrunt/Terraform through Docker.

Table of content:

* [Description](#desciption)
* [Configuration](#configuration)
* [Invocation arguments](#tgf-invocation)
* [Images](#default-docker-images)
* [Usage](#usage)

## Description

`TGF` allows is a small utility used to launch a Docker image and automatically map the current folder, your HOME folder and your current environment
variables to the underlying container.

By default, TGF is used as a frontend for [terragrunt](https://github.com/gruntwork-io/terragrunt), but it could also be used to run different endpoints.

### Why use TGF

Using `TGF` ensure that all your users are using the same set of tools to run infrastructure configuration even if they are working on different environments (`linux`, `Microsoft Windows`, `Mac OSX`, etc).

`Terraform` is very sensible to the version used and if one user update to a newer version, the state files will be marked with the latest version and
all other user will have to update their `Terraform` version to the latest used one.

Also, tools such as `AWS CLI` are updated on a regular basis and people don't tend to update their version regularly, resulting in many different version
among your users. If someone make a script calling a new feature of the `AWS` api, that script may break when executed by another user that has an
outdated version.

## Installation

Choose the desired version according to your OS [here](https://github.com/coveo/tgf/releases)

or install it through command line:

On `OSX`:

```bash
curl https://github.com/coveo/tgf/releases/download/v1.12/tgf_darwin -o /usr/local/bin/tgf && chmod +x /usr/local/bin/tgf
```

On `Linux`:

```bash
curl https://github.com/coveo/tgf/releases/download/v1.12/tgf_linux -o /usr/local/bin/tgf && chmod +x /usr/local/bin/tgf
```

On `Windows` with Powershell:

```powershell
Invoke-WebRequest https://github.com/coveo/tgf/releases/download/v1.12/tgf.exe -OutFile tgf.exe
```

## Configuration

TGF looks for a file named .tgf.config in the current working folder (and recursively in any parent folders) to get its parameters. If some parameters
are missing, it tries to find the remaining configuration through the [AWS parameter store](https://aws.amazon.com/ec2/systems-manager/parameter-store/)
under `/default/tgf` using your current [AWS CLI configuration](http://docs.aws.amazon.com/cli/latest/userguide/cli-chap-getting-started.html) if any.

Your configuration file could be expressed in any of the [YAML](http://www.yaml.org/start.html), [JSON](http://www.json.org/) or [Terraform HCL](https://www.terraform.io/docs/configuration/syntax.html) declarative language.

Example of YAML configuration file:

```text
docker-image: coveo/tgf
docker-refresh: 1h
logging-level: notice
entry-point: terragrunt
```

Example of HCL configuration file:

```text
docker-image = "coveo/tgf"
docker-refresh = "1h"
logging-level = "notice"
entry-point = "terragrunt"
```

Example of JSON configuration file:

```text
"docker-image": "coveo/tgf"
"docker-refresh": "1h"
"logging-level": "notice"
"entry-point": "terragrunt"
```

### Configuration keys

Key | Description | Default value
--- | --- | ---
| docker-image | Identify the docker image (with tag if necessary) to use | coveo/tgf
| docker-refresh | Delay before checking if a newer version of the docker image is available | 1h (1 hour)
| logging-level | Terragrunt logging level (only apply to Terragrunt entry point).<br>*Critical (0), Error (1), Warning (2), Notice (3), Info (4), Debug (5)* | Notice
| entry-point | The program that will be automatically launched when the docker starts | terragrunt
| tgf-recommended-version | The minimal tgf version recommended in your context | *no default*

Note: *The key names are not case sensitive*

## TGF Invocation

```text
> tgf
usage: tgf [<flags>]

tgf v1.12, a docker frontend for terragrunt. Any parameter after -- will be directly sent to the command identified by entrypoint.

Flags:
  -h, --help                   Show context-sensitive help (also try --help-long and --help-man).
  -e, --entrypoint=ENTRYPOINT  Override the entry point for docker whish is terragrunt by default
  -i, --image=IMAGE            Use the specified image instead of the default one
  -t, --tag=TAG                Use a different tag on docker image instead of the default one
  -r, --refresh                Force a refresh of the docker image
  -v, --version                Get the current version of tgf
```

If any of the tgf arguments conflicts with an argument of the desired entry point, you must place that argument after -- to ensure that they are
not interpreted by tgf and are passed to the entry point. Any non conflicting argument will be passed to the entry point wherever it is located on
the invocation arguments.

Example:

```bash
> tgf --version
v1.12
```

Returns the current version of the tgf tool

```bash
> tgf -- --version
terragrunt version v0.12.24.01(Coveo)
```

Returns the version of the default entry point (i.e. `Terragrunt`), the --version located after the -- instructs tgf to pass this argument
to the desired entry point

```bash
> tgf -e terraform -- --version
Terraform v0.9.11
```

Returns the version of `Terraform` since we specified the entry point to be terraform.

## Default Docker images

### Base image: coveo/tgf.base (based on Alpine)

* [Terraform](https://www.terraform.io/)
* [Terragrunt](https://github.com/coveo/terragrunt)
* [Go Template](https://github.com/coveo/gotemplate)
* Shells
  * `sh`

### Default image: coveo/tgf (based on Alpine)

All tools included in `coveo/tgf.base` plus:

* [Python](https://www.python.org/) (2 and 3)
* [Ruby](https://www.ruby-lang.org/en/)
* [AWS CLI](https://aws.amazon.com/cli/)
* [jq](https://stedolan.github.io/jq/)
* [Terraforming](https://github.com/dtan4/terraforming)
* [Tflint](https://github.com/wata727/tflint)
* [Terraform Quantum Provider](https://github.com/coveo/terraform-provider-quantum)
* Shells
  * `bash`
  * `zsh`
  * `fish`

### Full image: coveo/tgf.full (based on Ubuntu)

All tools included in `coveo/tgf` plus:

* [AWS Tools for Powershell](https://aws.amazon.com/powershell/)
* [Oh My ZSH](http://ohmyz.sh/)
* Shells
  * `powershell`

## Usage

### As Terragrunt front-end

```bash
> tgf plan
```

Invoke `terragrunt plan` (which will invoke `terraform plan`) after doing the `terragrunt` relative configurations.

```bash
> tgf apply -var env=dev
```

Invoke `terragrunt apply` (which will invoke `terraform apply`) after doing the `terragrunt` relative configurations. You can pass any arguments
that are supported by `terraform`.

```bash
> tgf plan-all
```

Invoke `terragrunt plan-all` (which will invoke `terraform plan` on the current folder and all sub folders). `Terragrunt` allows xxx-all operations to be
executed according to dependencies that are defined by the [dependencies statements](https://github.com/coveo/terragrunt#dependencies-between-modules).

### Other usages

```bash
> tgf -e aws s3 ls
```

Invoke `AWS CLI` as entry point and list all s3 buckets

```bash
> tgf -e fish
```

Invoke `AWS CLI` as entry point and list all s3 buckets

```bash
> tgf -t full -e powershell
```

Starts a `powershell` in the current working directory.

```bash
> tgf -e my_command -i my_image:latest
```

Invokes `my_command` in your own docker image. As you can see, you can do whatever you need to with `tgf`. It is not restricted to only the pre-packaged
Docker images, you can use it to run any program in any Docker images. Your imagination is your limit.
