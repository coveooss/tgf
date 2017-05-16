package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
)

func main() {
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	config := getDefaultValues()
	if lastRefresh(config.Image) > config.Refresh || !checkImage(config.Image) {
		refreshImage(config.Image)
	}
	callDocker(config.Image, config.LogLevel)
}

func callDocker(image string, logLevel string) {
	curDir, _ := os.Getwd()

	command := []string{"terragrunt"}
	command = append(command, os.Args[1:]...)
	command = append(command, []string{"--terragrunt-logging-level", logLevel}...)

	args := []string{
		"run", "-it",
		"-v", "/:/local",
		"-w", "/local/" + curDir,
		"-e", "HOME=/local" + os.Getenv("HOME"),
		"--name", "tgf.run",
		"--rm",
		image,
	}

	dockerCmd := exec.Command("docker", append(args, command...)...)
	dockerCmd.Stdin, dockerCmd.Stdout, dockerCmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	dockerCmd.Run()
}

func checkImage(image string) bool {
	var out bytes.Buffer
	dockerCmd := exec.Command("docker", []string{"images", "-q", image}...)
	dockerCmd.Stdout = &out
	dockerCmd.Run()
	return out.String() != ""
}

func refreshImage(image string) {
	var out bytes.Buffer
	dockerUpdateCmd := exec.Command("docker", "pull", image)
	dockerUpdateCmd.Stdout, dockerUpdateCmd.Stderr = &out, &out
	err := dockerUpdateCmd.Run()
	fmt.Fprintln(os.Stderr, out.String())
	PanicOnError(err)
	touchImageRefresh(image)
}
