terraform {
  required_version = ">= 0.12"
}

module "hello" {
  source = "/var/tgf/modules/hello"
}

output "mounted_message" {
  value = module.hello.message
}
