package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
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
}

func callDocker(imageParts ...string) {
	image := strings.Join(imageParts, ":")
	curDir, _ := os.Getwd()

	command := []string{"terragrunt"}
	command = append(command, os.Args[1:]...)

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
	command := []string{"docker"}

	args := []string{
		"images", "-q",
		image,
	}

	var out bytes.Buffer
	dockerCmd := exec.Command("docker", append(args, command...)...)
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
