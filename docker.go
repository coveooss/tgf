package main

import (
	"bytes"
	"fmt"
	"github.com/gruntwork-io/terragrunt/util"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strings"
)

func callDocker(image, logLevel, entryPoint string, args ...string) {
	command := append([]string{entryPoint}, args...)

	// Change the default log level for terragrunt
	const logLevelArg = "--terragrunt-logging-level"
	if !util.ListContainsElement(command, logLevelArg) && entryPoint == "terragrunt" {
		// The log level option should not be supplied if there is no actual command
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				command = append(command, []string{logLevelArg, logLevel}...)
				break
			}
		}
	}

	currentUser := Must(user.Current()).(*user.User)
	dockerArgs := []string{
		"run", "-it",
		"-v", "/:/local",
		"-v", fmt.Sprintf("%v:%v", currentUser.HomeDir, currentUser.HomeDir),
		"-e", fmt.Sprintf("HOME=%v", currentUser.HomeDir),
		"-w", filepath.Join("/local", Must(os.Getwd()).(string)),
		"--rm",
	}
	dockerArgs = append(dockerArgs, getEnviron()...)
	dockerArgs = append(dockerArgs, image)
	dockerArgs = append(dockerArgs, command...)

	dockerCmd := exec.Command("docker", dockerArgs...)
	dockerCmd.Stdin, dockerCmd.Stdout, dockerCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := dockerCmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "\n%s %s\n", dockerCmd.Path, strings.Join(dockerArgs, " "))
	}
}

func checkImage(image string) bool {
	var out bytes.Buffer
	dockerCmd := exec.Command("docker", []string{"images", "-q", image}...)
	dockerCmd.Stdout = &out
	dockerCmd.Run()
	return out.String() != ""
}

func refreshImage(image string) {
	fmt.Fprintf(os.Stderr, "Checking if there is a newer version of docker image %v\n", image)
	dockerUpdateCmd := exec.Command("docker", "pull", image)
	dockerUpdateCmd.Stdout, dockerUpdateCmd.Stderr = os.Stderr, os.Stderr
	err := dockerUpdateCmd.Run()
	PanicOnError(err)
	touchImageRefresh(image)
	fmt.Fprintln(os.Stderr)
}

func getEnviron() (result []string) {
	for _, env := range os.Environ() {
		split := strings.Split(env, "=")
		switch split[0] {
		case
			"PATH", "PYTHONPATH", "GOPATH",
			"_", "PWD", "OLDPWD", "TMPDIR",
			"PROMPT", "HOME", "SHELL", "SH", "ZSH",
			"DISPLAY", "TERM":
		default:
			result = append(result, "-e")
			result = append(result, split[0])
		}
	}
	return
}
