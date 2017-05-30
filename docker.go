package main

import (
	"bytes"
	"fmt"
	"github.com/gruntwork-io/terragrunt/util"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
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
	home := filepath.ToSlash(currentUser.HomeDir)
	homeWithoutVolume := strings.TrimPrefix(home, filepath.VolumeName(home))
	cwd := filepath.ToSlash(Must(os.Getwd()).(string))
	currentDrive := fmt.Sprintf("%s/", filepath.VolumeName(cwd))
	dockerArgs := []string{
		"run", "-it",
		"-v", fmt.Sprintf("%v:/local", convertDrive(currentDrive)),
		"-v", fmt.Sprintf("%v:%v", convertDrive(home), homeWithoutVolume),
		"-e", fmt.Sprintf("HOME=%v", homeWithoutVolume),
		"-w", util.JoinPath("/local", strings.TrimPrefix(cwd, filepath.VolumeName(cwd))),
		"--rm",
	}
	dockerArgs = append(dockerArgs, getEnviron()...)
	dockerArgs = append(dockerArgs, image)
	dockerArgs = append(dockerArgs, command...)

	dockerCmd := exec.Command("docker", dockerArgs...)
	dockerCmd.Stdin, dockerCmd.Stdout = os.Stdin, os.Stdout
	var stderr bytes.Buffer
	dockerCmd.Stderr = &stderr

	if err := dockerCmd.Run(); err != nil {
		if stderr.Len() > 0 {
			fmt.Fprintf(os.Stderr, stderr.String())
			fmt.Fprintf(os.Stderr, "\n%s %s\n", dockerCmd.Path, strings.Join(dockerArgs, " "))

			if runtime.GOOS == "windows" {
				fmt.Fprintln(os.Stderr, windowsMessage)
			}
		}
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
		varName := strings.TrimSpace(split[0])
		varUpper := strings.ToUpper(varName)
		if varName == "" || strings.Contains(varUpper, "PATH") {
			continue
		}

		if runtime.GOOS == "windows" {
			if strings.Contains(strings.ToUpper(split[1]), `C:\`) || strings.Contains(varUpper, "WIN") {
				continue
			}
		}

		switch varName {
		case
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

// This function set the path converter function
// For old Windows version still using docker-machine and VirtualBox,
// it transforms the C:\ to /C/.
func getPathConversionFunction() func(string) string {
	if runtime.GOOS != "windows" || os.Getenv("DOCKER_MACHINE_NAME") == "" {
		return func(path string) string { return path }
	}

	return func(path string) string {
		return fmt.Sprintf("/%s%s", strings.ToUpper(path[:1]), path[2:])
	}
}

var convertDrive = getPathConversionFunction()

var windowsMessage = `
You may have to share your drives with your Docker virtual machine to make them accessible.

On Windows 10+ using Hyper-V to run Docker, simply right click on Docker icon in your tray and
choose "Settings", then go to "Shared Drives" and enable the share for the drives you want to 
be accessible to your dockers.

On previous version using VirtualBox, start the VirtualBox application and add shared drives
for all drives you want to make shareable with your dockers.

IMPORTANT, to make your drives accessible to tgf, you have to give them uppercase name corresponding
to the drive letter:
	C:\ ==> /C
	D:\ ==> /D
	...
	Z:\ ==> /Z
`
