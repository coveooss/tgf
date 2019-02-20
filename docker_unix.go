// +build !windows

package main

import (
	"fmt"
	"os"
	"syscall"
)

const dockerSocketMountPattern = "%[1]s:%[1]s"

func getDockerGroup() string {
	s := must(os.Stat(dockerSocketFile)).(os.FileInfo)
	return fmt.Sprintf("%v", s.Sys().(*syscall.Stat_t).Gid)
}
