package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"

	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/util"
)

func callDocker(config tgfConfig, mapHome bool, flushCache bool, debug bool, dockerOptions []string, args ...string) int {
	command := append([]string{config.EntryPoint}, args...)

	// Change the default log level for terragrunt
	const logLevelArg = "--terragrunt-logging-level"
	if !util.ListContainsElement(command, logLevelArg) && config.EntryPoint == "terragrunt" {
		if config.LogLevel == "6" || strings.ToLower(config.LogLevel) == "full" {
			config.LogLevel = "debug"
			os.Setenv("TF_LOG", "DEBUG")
			os.Setenv("TERRAGRUNT_DEBUG", "1")
		}

		// The log level option should not be supplied if there is no actual command
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				command = append(command, []string{logLevelArg, config.LogLevel}...)
				break
			}
		}
	}

	if flushCache && config.EntryPoint == "terragrunt" {
		command = append(command, "--terragrunt-source-update")
	}

	currentUser := Must(user.Current()).(*user.User)
	home := filepath.ToSlash(currentUser.HomeDir)
	homeWithoutVolume := strings.TrimPrefix(home, filepath.VolumeName(home))

	cwd := filepath.ToSlash(Must(os.Getwd()).(string))
	currentDrive := fmt.Sprintf("%s/", filepath.VolumeName(cwd))
	rootFolder := strings.Split(strings.TrimPrefix(cwd, currentDrive), "/")[0]

	temp := filepath.ToSlash(filepath.Join(os.TempDir(), "tgf-cache"))
	tempDrive := fmt.Sprintf("%s/", filepath.VolumeName(temp))
	tempFolder := strings.TrimPrefix(temp, tempDrive)

	dockerArgs := []string{
		"run", "-it",
		"-v", fmt.Sprintf("%s%s:/%[2]s", convertDrive(currentDrive), rootFolder),
		"-v", fmt.Sprintf("%s%s:/var/tgf", convertDrive(tempDrive), tempFolder),
		"-w", strings.TrimPrefix(cwd, filepath.VolumeName(cwd)),
		"--rm",
	}
	if mapHome {
		dockerArgs = append(dockerArgs, []string{
			"-v", fmt.Sprintf("%v:%v", convertDrive(home), homeWithoutVolume),
			"-e", fmt.Sprintf("HOME=%v", homeWithoutVolume),
		}...)
	}

	os.Setenv("TERRAGRUNT_CACHE", "/var/tgf")
	os.Setenv("TGF_COMMAND", config.EntryPoint)
	os.Setenv("TGF_VERSION", version)
	os.Setenv("TGF_IMAGE", config.Image)
	os.Setenv("TGF_IMAGE_VERSION", config.ImageVersion)
	os.Setenv("TGF_IMAGE_TAG", config.ImageTag)
	os.Setenv("TGF_IMAGE_NAME", config.GetImageName())

	for _, do := range dockerOptions {
		dockerArgs = append(dockerArgs, strings.Split(do, " ")...)
	}
	dockerArgs = append(dockerArgs, getEnviron(mapHome)...)
	dockerArgs = append(dockerArgs, config.GetImageName())
	dockerArgs = append(dockerArgs, command...)
	dockerCmd := exec.Command("docker", dockerArgs...)
	dockerCmd.Stdin, dockerCmd.Stdout = os.Stdin, os.Stdout
	var stderr bytes.Buffer
	dockerCmd.Stderr = &stderr

	if debug {
		debugString := color.New(color.FgWhite, color.Faint).SprintfFunc()
		fmt.Fprint(os.Stderr, debugString("%s\n\n", strings.Join(dockerCmd.Args, " ")))
	}

	if err := dockerCmd.Run(); err != nil {
		if stderr.Len() > 0 {
			fmt.Fprintf(os.Stderr, stderr.String())
			fmt.Fprintf(os.Stderr, "\n%s %s\n", dockerCmd.Args[0], strings.Join(dockerArgs, " "))

			if runtime.GOOS == "windows" {
				fmt.Fprintln(os.Stderr, windowsMessage)
			}
		}
	}
	return dockerCmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
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

func getEnviron(mapHome bool) (result []string) {
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
			"PROMPT", "SHELL", "SH", "ZSH", "HOME",
			"LANG", "LC_CTYPE", "DISPLAY", "TERM":
		case "LOGNAME", "USER":
			if !mapHome {
				continue
			}
			fallthrough
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
