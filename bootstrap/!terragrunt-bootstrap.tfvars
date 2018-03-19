# By default, sources are located in the same directory as the config file
source_path = ""

terragrunt = {
  description = <<EOF
  Terragrunt default configuration bootstrap
  ------------------------------------------

  Configures some additional Terragrunt command to enhance the user experience with Terraform.
  EOF

  terraform {
    # Generic path to retrieve the terraform source relatively to the terragrunt deployment folder
    # Can be overridden in team project. Note that the double slash // is intended, that indicates
    # to include the whole structure under modules instead of just the target directory.
    # See terraform documentation on source include https://www.terraform.io/docs/modules/sources.html
    # for more information.
    source = "${get_parent_tfvars_dir()}/${var.source_path}/${path_relative_to_include()}"

    # Force Terraform to keep trying to acquire a lock for up to 20 minutes if someone else already has the lock.
    extra_arguments lock-timeout {
      display_name = "Add lock timeout on terraform commands"
      commands     = ["${get_terraform_commands_that_need_locking()}"]
      arguments    = ["-lock-timeout=20m"]
    }

    # Ideally, user shall never have to provide input to terraform / terragrunt / tgf command
    # so undefined values should be considered as error.
    extra_arguments disable-input {
      display_name = "Disable input on terraform commands"
      commands     = ["${get_terraform_commands_that_need_input()}"]
      arguments    = ["-input=false"]
    }

    # Include variables
    extra_arguments include-variables {
      display_name = "Automatically include variables"
      commands     = ["${get_terraform_commands_that_need_vars()}"]
      arguments    = ["-var-file=${save_variables("_variables.tfvars")}"]
    }
  }

  #-------------------------------------------------------------------------------------------------------
  # Add extra commands to terragrunt

  # Various shells
  extra_command shells {
    display_name = "Allow launching of command shells"

    description = <<EOF
    Run the specified shell instead of running terraform.

    This is useful to get information about the current context.

    Ex:
      tgf bash                              # Start a shell in the temp folder
      tgf shell
      tgf sh

      tgf fish                              # Start a fish shell in the temp folder
      tgf powershell                        # Start a powershell (requires full image)
      tgf pwsh
    EOF

    commands      = ["bash", "zsh", "fish", "pwsh"]
    aliases       = ["sh", "shell", "powershell=pwsh"]
    shell_command = true
    use_state     = true
    ignore_error  = true

    version = <<-EOF
    #! /usr/bin/env bash
    $TERRAGRUNT_COMMAND --version | head -n1
    exit ${PIPESTATUS[0]}
    EOF

    os = ["darwin", "linux"]
  }
  #
  # Other shell command
  extra_command commands {
    display_name = "Allow launching of standard shell commands"

    description = <<EOF
    Run the specified command instead of running terraform.

    This is useful to get information about the current context.

    Ex:
      tgf ls -l                   # List the content of the temp folder
      tgf cat _auto_variables.tf  # Get the content of the auto variables file
      tgf pwd                     # Get the name of the temporary folder where
                                  # terraform is executed
    EOF

    commands     = ["ls", "cat", "echo", "pwd", "find", "uname", "id", "du", "tree"]
    aliases      = ["env=env | sort"]
    os           = ["darwin", "linux"]
    ignore_error = true
  }
  #
  # Command 
  extra_command "prepare-state" {
    display_name = "Preparation of the state file"

    description = <<EOF
    Prepare the environment by importing, removing, renaming resources to/from your state file.

    Type 'tgf prepare -h' to get help.
    EOF

    command = "prepare-terraform-state"
    aliases = ["prepare"]
    os      = ["darwin", "linux"]
  }
  #
  # Printouts variables from Terragrunt current context
  extra_command "get-variables" {
    display_name = "Get available variables"
    commands     = ["cat"]
    arguments    = ["_variables.tfvars"]
    aliases      = ["var", "vars", "variable", "variables"]
    os           = ["darwin", "linux"]
    ignore_error = true
  }
  #
  extra_command "utilities" {
    display_name = "Scripting & Utilities"
    description  = "Scripting languages and other tools to help running your pre and post hooks"
    commands     = ["python2", "python3", "ruby", "jq", "git", "hg"]
    use_state    = false
    ignore_error = true
    version      = "--version"
  }
  #
  extra_command "aws" {
    display_name = "AWS CLI commands"

    description = <<EOF
    See AWS CLI web site on http://docs.aws.amazon.com/cli/latest/reference/ for more information).

    Type 'tgf aws help' to get help.
    EOF

    commands     = ["aws", "s3cat"]
    aliases      = ["cats3=s3cat", "s3cat=s3cat", "s3=aws s3"]
    use_state    = false
    ignore_error = true
    version      = "[ $TERRAGRUNT_COMMAND == 'aws' ] && aws --version"
  }
  #
  extra_command "tflint" {
    display_name = "Apply TF linter"
    description  = "Execute tflint on the current directory to do a more extensive validation."
    aliases      = ["lint"]
    arguments    = ["--var-file=_variables.tfvars"]
    version      = "--version"
    os           = ["darwin", "linux"]
  }
  #
  extra_command "terraforming" {
    display_name = "Run terraforming"

    description = <<EOF
    Call the terraforming tools to generate terraform code from existing resources.
    See https://github.com/dtan4/terraforming/blob/master/README.md to get the documentation.
    EOF

    aliases = ["forming"]
    os      = ["darwin", "linux"]
  }
  #
  extra_command "gotemplate" {
    display_name = "Run gotemplate"
    description  = "Invoke the gotemplate tool"
    aliases      = ["gt"]

    version = <<-EOF
    #! /usr/bin/env bash
    printf 'gotemplate %s' $($TERRAGRUNT_COMMAND --version)
    EOF
  }
  #
  extra_command "terraform-docs" {
    description = "Utility to extract the documentation from your terraform source code"
    commands    = ["terraform-docs", "get-documentation"]
    aliases     = ["doc=get-documentation", "docs=get-documentation"]
    use_state   = false
    os          = ["darwin", "linux"]

    version = <<-EOF
    #! /usr/bin/env bash
    if [ $TERRAGRUNT_COMMAND == 'terraform-docs' ]
    then
      printf 'terraform-docs %s' $($TERRAGRUNT_COMMAND --version)
    fi
    EOF
  }
  #
  extra_command "readable-plan" {
    display_name = "Enhanced terraform plan"

    description = <<EOF
    Execute terraform plan with enhanced output.
    Note that this overrides the default 'plan' command.
    EOF

    aliases = ["plan", "plan-r"]
    act_as  = "plan"
    os      = ["darwin", "linux"]

    command = <<-EOF
    #! /usr/bin/env bash
    ${TERRAGRUNT_TFPATH:-terraform} plan $* 2>&1 | readable-output.py
    exit ${PIPESTATUS[0]}
    EOF
  }
  #
  extra_command "readable-apply" {
    display_name = "Enhanced terraform apply"

    description = <<EOF
    Execute terraform apply with enhanced output.
    Note that this overrides the default 'apply' command.
    EOF

    aliases = ["apply", "apply-r"]
    act_as  = "apply"
    os      = ["darwin", "linux"]

    command = <<-EOF
    #! /usr/bin/env bash
    ${TERRAGRUNT_TFPATH:-terraform} apply $* 2>&1 | readable-output.py
    exit ${PIPESTATUS[0]}
    EOF
  }
  #
  extra_command "plan-native" {
    display_name = "Native terraform plan"
    description  = "Execute the native 'terraform plan' command without output enhancement (as delivered by HashiCorp)."
    commands     = ["${get_env("TERRAGRUNT_TFPATH", "terraform")}"]
    arguments    = ["plan"]
    aliases      = ["plan-tf", "plan"]
    act_as       = "plan"
  }
  #
  extra_command "apply-native" {
    display_name = "Native terraform apply"
    description  = "Execute the native 'terraform apply' command without output enhancement (as delivered by HashiCorp)."
    commands     = ["${get_env("TERRAGRUNT_TFPATH", "terraform")}"]
    arguments    = ["apply"]
    aliases      = ["apply-tf", "apply"]
    act_as       = "apply"
  }
  #
  extra_command "terraform-force-unlock" {
    display_name = "Automated terraform unlock"
    description  = "Automatically unlock the current project if is is locked"
    os           = ["darwin", "linux"]
    aliases      = ["force-unlock", "unlock"]
    act_as       = "force-unlock"

    command = <<-EOF
    #! /usr/bin/env bash
    set -- $(${TERRAGRUNT_TFPATH:-terraform} force-unlock -force 0 2>&1 | grep "ID:")
    if [ -n "$2" ]
    then
        ${TERRAGRUNT_TFPATH:-terraform} force-unlock -force $2
    else
        echo "This project is not locked"    
    fi
    EOF
  }
  #
  extra_command "init" {
    display_name = "Customized terraform init"

    description = <<EOF
    Override the behavior of Terragrunt which does not let the user to call 'init'.

    IMPORTANT: You should never use that command to reconfigure the backend.
               This command does not call 'terraform init', it calls 'rm'
    EOF

    commands     = ["rm"]
    arguments    = ["-rf", ".terraform", "$TERRAGRUNT_CACHE_FOLDER"]
    aliases      = ["flush-cache", "flush"]
    act_as       = "rm"
    use_state    = false
    ignore_error = true
    os           = ["darwin", "linux"]
  }
  #
  extra_command "list-cache" {
    display_name = "List terragrunt cache content"
    description  = "List the content of the Terragrunt cache folder"
    commands     = ["ls"]
    arguments    = ["-lR", "$TERRAGRUNT_CACHE_FOLDER"]
    aliases      = ["cache"]
    use_state    = false
    ignore_error = true
    os           = ["darwin", "linux"]
  }
  #
  extra_command "get-bootstrap" {
    display_name = "Display bootstrap content"
    description  = "Display the content of this bootstrap file"
    commands     = ["s3cat"]
    arguments    = ["s3://${var.modules_bucket}/bootstrap/${var.env}/${var.region}/infra/terraform.tfvars"]
    aliases      = ["bootstrap", "boot"]
    use_state    = false
    ignore_error = true
    os           = ["darwin", "linux"]
  }
  #
  #-------------------------------------------------------------------------------------------------------
  # Commands that are executed before execution of the actual terraform command
  # Can be disabled or overridden by supplying a pre_hook section with the exact same name
  #
  pre_hook "apply-gotemplate" {
    display_name = "Apply go template"

    description = <<EOF
    Run 'gotemplate' recursively on all files with the following extensions: .gt & .template.tf
    
    See https://github.com/coveo/gotemplate/blob/master/README.md for further information or
    type the following commands: 
      tgf gt -h
      tgf gt run -h
    EOF

    command = <<-EOF
    #! /usr/bin/env bash
    gotemplate -or --color -L $TERRAGRUNT_LOGGING_LEVEL -i _variables.tfvars "$@"
    EOF

    arguments = ["--exclude=${var.gotemplate_excluded_patterns}"]
  }
}
