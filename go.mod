module github.com/coveo/tgf

go 1.12

replace github.com/gruntwork-io/terragrunt => github.com/coveo/terragrunt v1.3.3

require (
	github.com/aws/aws-sdk-go v1.19.21
	github.com/blang/semver v3.5.1+incompatible
	github.com/coveo/gotemplate v2.7.6+incompatible // indirect
	github.com/coveo/gotemplate/v3 v3.0.2
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fatih/color v1.7.0
	github.com/gruntwork-io/terragrunt v0.0.0-00010101000000-000000000000
	github.com/hashicorp/go-getter v1.2.0
	github.com/hashicorp/terraform v0.11.13 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/pkg/errors v0.8.1 // indirect
	gopkg.in/alecthomas/kingpin.v2 v2.2.6
	gopkg.in/matryer/try.v1 v1.0.0-20150601225556-312d2599e12e // indirect
	gopkg.in/yaml.v2 v2.2.2
)
