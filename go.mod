module github.com/coveo/tgf

go 1.12

replace github.com/gruntwork-io/terragrunt => github.com/coveo/terragrunt v1.4.0-beta1

require (
	github.com/aws/aws-sdk-go v1.19.31
	github.com/blang/semver v3.5.1+incompatible
	github.com/coveo/gotemplate/v3 v3.1.0-test
	github.com/coveo/kingpin/v2 v2.3.0
	github.com/docker/distribution v2.7.1+incompatible // indirect
	github.com/docker/docker v1.13.1
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.4.0 // indirect
	github.com/fatih/color v1.7.0
	github.com/gruntwork-io/terragrunt v0.0.0-00010101000000-000000000000
	github.com/hashicorp/go-getter v1.2.0
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/stretchr/testify v1.3.0
	gopkg.in/yaml.v2 v2.2.2
)
