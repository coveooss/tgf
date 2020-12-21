// +build linux

package main

import (
	"fmt"
	"os"
	"syscall"
)

const dockerSocketMountPattern = "%[1]s:%[1]s"

func getDockerMountArgs() []string {
	return []string{"-v", getDockerSocketMount(), "--group-add", getDockerGroup()}
}

func getDockerSocketMount() string {
	return fmt.Sprintf(dockerSocketMountPattern, dockerSocketFile)
}

func getDockerGroup() string {
	s := must(os.Stat(dockerSocketFile)).(os.FileInfo)
	return fmt.Sprintf("%v", s.Sys().(*syscall.Stat_t).Gid)
}
