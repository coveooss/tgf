package main

import (
	"fmt"
)

func getDockerMountArgs() []string {
	// MacOS has peculiar permissions, so mounting /var/run/docker.sock doesn't work.
	// See: https://github.com/docker/for-mac/issues/4755#issuecomment-726351209
	// We mount the raw socket directly from the VM, which has group 'root' in the VM, so we add this group to the user.
	return []string{"-v", getDockerSocketMount(), "--group-add", "root"}
}

func getDockerSocketMount() string {
	return fmt.Sprintf("%[1]s.raw:%[1]s", dockerSocketFile)
}
