// +build linux

package main

import (
	"github.com/coveo/gotemplate/utils"
	"os"
	"strings"
)

const dockerSocketMountPattern = "%[1]s:%[1]s"

func getDockerGroup() string {
	cmd, tempFile, err := utils.GetCommandFromString("stat -c '%g' " + dockerSocketFile)
	if err != nil {
		panic(err)
	}
	if tempFile != "" {
		defer func() { os.Remove(tempFile) }()
	}
	return strings.TrimSpace(string(must(cmd.Output()).([]byte)))
}
