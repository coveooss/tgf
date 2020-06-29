package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"os/signal"
	"os/user"
	"path"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"syscall"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ecr"
	"github.com/blang/semver"
	"github.com/coveooss/gotemplate/v3/utils"
	"github.com/coveooss/multilogger/reutils"
	"github.com/coveooss/terragrunt/v2/util"
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const (
	minimumDockerVersion = "1.25"
	tgfImageVersion      = "TGF_IMAGE_VERSION"
	dockerSocketFile     = "/var/run/docker.sock"
	dockerfilePattern    = "TGF_dockerfile"
	maxDockerTagLength   = 128
)

type dockerConfig struct{ *TGFConfig }

func (docker *dockerConfig) call() int {
	app, config := docker.tgf, docker.TGFConfig
	args := app.Unmanaged
	command := append(strings.Split(config.EntryPoint, " "), args...)

	// Change the default log level for terragrunt
	const logLevelArg = "--terragrunt-logging-level"
	if !util.ListContainsElement(command, logLevelArg) && filepath.Base(config.EntryPoint) == "terragrunt" {
		level, _ := strconv.Atoi(config.LogLevel)
		if level > 6 || strings.ToLower(config.LogLevel) == "full" {
			config.LogLevel = "trace"
			config.Environment["TF_LOG"] = "TRACE"
			config.Environment["TERRAGRUNT_DEBUG"] = "1"
		}

		// The log level option should not be supplied if there is no actual command
		for _, arg := range args {
			if !strings.HasPrefix(arg, "-") {
				command = append(command, []string{logLevelArg, config.LogLevel}...)
				break
			}
		}
	}

	if app.FlushCache && filepath.Base(config.EntryPoint) == "terragrunt" {
		command = append(command, "--terragrunt-source-update")
	}

	imageName := docker.getImage()

	if app.GetImageName {
		Println(imageName)
		return 0
	}

	cwd := filepath.ToSlash(must(filepath.EvalSymlinks(must(os.Getwd()).(string))).(string))
	currentDrive := fmt.Sprintf("%s/", filepath.VolumeName(cwd))
	rootFolder := strings.Split(strings.TrimPrefix(cwd, currentDrive), "/")[0]
	sourceFolder := fmt.Sprintf("/%s", filepath.ToSlash(strings.Replace(strings.TrimPrefix(cwd, currentDrive), rootFolder, app.MountPoint, 1)))

	dockerArgs := []string{
		"run",
	}
	if app.DockerInteractive {
		dockerArgs = append(dockerArgs, "-it")
	}
	dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s%s:/%s", convertDrive(currentDrive), rootFolder, app.MountPoint), "-w", sourceFolder)

	if app.WithDockerMount {
		withDockerMountArgs := []string{"-v", fmt.Sprintf(dockerSocketMountPattern, dockerSocketFile), "--group-add", getDockerGroup()}
		dockerArgs = append(dockerArgs, withDockerMountArgs...)
	}

	// No need to map to current user on windows. Files written by docker containers in windows seem to be accessible by the user calling docker
	if app.WithCurrentUser && runtime.GOOS != "windows" {
		currentUser := must(user.Current()).(*user.User)
		dockerArgs = append(dockerArgs, fmt.Sprintf("--user=%s:%s", currentUser.Uid, currentUser.Gid))
	}

	if app.MountHomeDir {
		currentUser := must(user.Current()).(*user.User)
		home := filepath.ToSlash(currentUser.HomeDir)
		mountingHome := fmt.Sprintf("/home/%s", filepath.Base(home))

		dockerArgs = append(dockerArgs, []string{
			"-v", fmt.Sprintf("%v:%v", convertDrive(home), mountingHome),
			"-e", fmt.Sprintf("HOME=%v", mountingHome),
		}...)

		dockerArgs = append(dockerArgs, config.DockerOptions...)
	}

	if app.MountTempDir {
		temp := filepath.ToSlash(filepath.Join(must(filepath.EvalSymlinks(os.TempDir())).(string), "tgf-cache"))
		tempDrive := fmt.Sprintf("%s/", filepath.VolumeName(temp))
		tempFolder := strings.TrimPrefix(temp, tempDrive)
		if runtime.GOOS == "windows" {
			os.Mkdir(temp, 0755)
		}
		dockerArgs = append(dockerArgs, "-v", fmt.Sprintf("%s%s:/var/tgf", convertDrive(tempDrive), tempFolder))
		config.Environment["TGF_TEMP_FOLDER"] = path.Join(tempDrive, tempFolder)
		config.Environment["TERRAGRUNT_CACHE"] = "/var/tgf"
	}

	config.Environment["TGF_COMMAND"] = config.EntryPoint
	config.Environment["TGF_VERSION"] = version
	config.Environment["TGF_ARGS"] = strings.Join(os.Args, " ")
	config.Environment["TGF_LAUNCH_FOLDER"] = sourceFolder
	config.Environment["TGF_IMAGE_NAME"] = imageName // sha256 of image

	if !strings.Contains(config.Image, "coveo/tgf") { // the tgf image injects its own image info
		config.Environment["TGF_IMAGE"] = config.Image
		if config.ImageVersion != nil {
			config.Environment[tgfImageVersion] = *config.ImageVersion
			if version, err := semver.Make(*config.ImageVersion); err == nil {
				config.Environment["TGF_IMAGE_MAJ_MIN"] = fmt.Sprintf("%d.%d", version.Major, version.Minor)
			}
		}
		if config.ImageTag != nil {
			config.Environment["TGF_IMAGE_TAG"] = *config.ImageTag
		}
	}

	for key, val := range config.Environment {
		os.Setenv(key, val)
		app.Debug("export %v=%v", key, val)
	}

	for _, do := range app.DockerOptions {
		dockerArgs = append(dockerArgs, strings.Split(do, " ")...)
	}

	if !util.ListContainsElement(dockerArgs, "--name") {
		// We do not remove the image after execution if a name has been provided
		dockerArgs = append(dockerArgs, "--rm")
	}

	dockerArgs = append(dockerArgs, getEnviron(app.MountHomeDir)...)
	dockerArgs = append(dockerArgs, imageName)
	dockerArgs = append(dockerArgs, command...)
	dockerCmd := exec.Command("docker", dockerArgs...)
	dockerCmd.Stdin, dockerCmd.Stdout = os.Stdin, os.Stdout
	var stderr bytes.Buffer
	dockerCmd.Stderr = &stderr

	if len(config.Environment) > 0 {
		app.Debug("")
	}
	app.Debug("%s\n", strings.Join(dockerCmd.Args, " "))

	if err := runCommands(config.runBeforeCommands); err != nil {
		return -1
	}
	if err := dockerCmd.Run(); err != nil {
		if stderr.Len() > 0 {
			ErrPrintf(errorString(stderr.String()))
			ErrPrintf("\n%s %s\n", dockerCmd.Args[0], strings.Join(dockerArgs, " "))

			if runtime.GOOS == "windows" {
				ErrPrintln(windowsMessage)
			}
		}
	}
	if err := runCommands(config.runAfterCommands); err != nil {
		ErrPrintf(errorString("%v", err))
	}

	return dockerCmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus()
}

func runCommands(commands []string) error {
	for _, script := range commands {
		cmd, tempFile, err := utils.GetCommandFromString(script)
		if err != nil {
			return err
		}
		if tempFile != "" {
			defer func() { os.Remove(tempFile) }()
		}
		cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			return err
		}
	}
	return nil
}

// Returns the image name to use
// If docker-image-build option has been set, an image is dynamically built and the resulting image digest is returned
func (docker *dockerConfig) getImage() (name string) {
	app := docker.tgf
	name = docker.GetImageName()
	if !strings.Contains(name, ":") {
		name += ":latest"
	}

	if !app.DockerBuild {
		return
	}

	lastHash := ""
	for i, ib := range docker.imageBuildConfigs {
		var temp, folder, dockerFile string
		var out *os.File
		if ib.Folder == "" {
			// There is no explicit folder, so we create a temporary folder to store the docker file
			app.Debug("Creating build folder")
			temp = must(ioutil.TempDir("", "tgf-dockerbuild")).(string)
			out = must(os.Create(filepath.Join(temp, dockerfilePattern))).(*os.File)
			folder = temp
		} else {
			if ib.Instructions != "" {
				app.Debug("Creating dockerfile in provider build folder")
				out = must(ioutil.TempFile(ib.Dir(), dockerfilePattern)).(*os.File)
				temp = out.Name()
				dockerFile = temp
			}
			folder = ib.Dir()
		}

		if out != nil {
			app.Debug("Writing instructions to dockerfile")
			ib.Instructions = fmt.Sprintf("FROM %s\n%s\n", name, ib.Instructions)
			must(fmt.Fprintf(out, ib.Instructions))
			must(out.Close())
		}

		if temp != "" {
			// A temporary file of folder has been created, we register functions to ensure proper cleanup
			cleanup := func() { os.RemoveAll(temp) }
			defer cleanup()
			c := make(chan os.Signal, 1)
			signal.Notify(c, os.Interrupt, syscall.SIGTERM)
			go func() {
				<-c
				Println("\nRemoving file", dockerFile)
				cleanup()
				panic(errorString("Execution interrupted by user: %v", c))
			}()
		}

		// We remove the last hash from the name to avoid cumulating several hash in the final name
		name = strings.Replace(name, lastHash, "", 1)
		lastHash = fmt.Sprintf("-%s", ib.hash())

		name = name + "-" + ib.GetTag()
		if image, tag := Split2(name, ":"); len(tag) > maxDockerTagLength {
			name = image + ":" + tag[0:maxDockerTagLength]
		}
		if app.Refresh || getImageHash(name) != ib.hash() {
			label := fmt.Sprintf("hash=%s", ib.hash())
			args := []string{"build", ".", "-f", dockerfilePattern, "--quiet", "--force-rm", "--label", label}
			if i == 0 && app.Refresh && !app.UseLocalImage {
				args = append(args, "--pull")
			}
			if dockerFile != "" {
				args = append(args, "--file")
				args = append(args, filepath.Base(dockerFile))
			}

			args = append(args, "--tag", name)
			buildCmd := exec.Command("docker", args...)

			app.Debug("%s", strings.Join(buildCmd.Args, " "))
			if ib.Instructions != "" {
				app.Debug("%s", ib.Instructions)
			}
			buildCmd.Stderr = os.Stderr
			buildCmd.Dir = folder
			must(buildCmd.Output())
			pruneDangling()
		}
	}

	return
}

var pruneDangling = func() {
	cli, ctx := getDockerClient()
	danglingFilters := filters.NewArgs()
	danglingFilters.Add("dangling", "true")
	if _, err := cli.ImagesPrune(ctx, danglingFilters); err != nil {
		printError("Error pruning dangling images (Untagged): %v", err.Error())
	}
	if _, err := cli.ContainersPrune(ctx, filters.Args{}); err != nil {
		printError("Error pruning unused containers: %v", err.Error())
	}
}

func (docker *dockerConfig) prune(images ...string) {
	cli, ctx := getDockerClient()
	if len(images) > 0 {
		current := fmt.Sprintf(">=%s", docker.GetActualImageVersion())
		for _, image := range images {
			filters := filters.NewArgs()
			filters.Add("reference", image)
			if images, err := cli.ImageList(ctx, types.ImageListOptions{Filters: filters}); err == nil {
				for _, image := range images {
					actual := getActualImageVersionFromImageID(image.ID)
					if actual == "" {
						for _, tag := range image.RepoTags {
							matches, _ := reutils.MultiMatch(tag, reImage)
							if version := matches["version"]; version != "" {
								if len(version) > len(actual) {
									actual = version
								}
							}
						}
					}
					upToDate, err := CheckVersionRange(actual, current)
					if err != nil {
						ErrPrintln("Check version for %s vs%s: %v", actual, current, err)
					} else if !upToDate {
						for _, tag := range image.RepoTags {
							deleteImage(tag)
						}
					}
				}
			}
		}
	}
	pruneDangling()
}

func deleteImage(id string) {
	cli, ctx := getDockerClient()
	items, err := cli.ImageRemove(ctx, id, types.ImageRemoveOptions{})
	if err != nil {
		printError((err.Error()))
	}
	for _, item := range items {
		if item.Untagged != "" {
			ErrPrintf("Untagged %s\n", item.Untagged)
		}
		if item.Deleted != "" {
			ErrPrintf("Deleted %s\n", item.Deleted)
		}
	}
}

// GetActualImageVersion returns the real image version stored in the environment variable TGF_IMAGE_VERSION
func (docker *dockerConfig) GetActualImageVersion() string {
	return getActualImageVersionInternal(docker.getImage())
}

func getDockerClient() (*client.Client, context.Context) {
	if dockerClient == nil {
		os.Setenv("DOCKER_API_VERSION", minimumDockerVersion)
		dockerClient = must(client.NewEnvClient()).(*client.Client)
		dockerContext = context.Background()
	}
	return dockerClient, dockerContext
}

var dockerClient *client.Client
var dockerContext context.Context

func getImageSummary(imageName string) *types.ImageSummary {
	cli, ctx := getDockerClient()
	// Find image
	filters := filters.NewArgs()
	filters.Add("reference", imageName)
	images, err := cli.ImageList(ctx, types.ImageListOptions{Filters: filters})
	if err != nil || len(images) != 1 {
		return nil
	}
	return &images[0]
}

func getActualImageVersionInternal(imageName string) string {
	if image := getImageSummary(imageName); image != nil {
		return getActualImageVersionFromImageID(image.ID)
	}
	return ""
}

func getImageHash(imageName string) string {
	if image := getImageSummary(imageName); image != nil {
		return image.Labels["hash"]
	}
	return ""
}

func getActualImageVersionFromImageID(imageID string) string {
	cli, ctx := getDockerClient()
	inspect, _, err := cli.ImageInspectWithRaw(ctx, imageID)
	if err != nil {
		panic(err)
	}
	for _, v := range inspect.ContainerConfig.Env {
		values := strings.SplitN(v, "=", 2)
		if values[0] == tgfImageVersion {
			return values[1]
		}
	}
	// We do not found an environment variable with the version in the images
	return ""
}

func checkImage(image string) bool {
	var out bytes.Buffer
	dockerCmd := exec.Command("docker", []string{"images", "-q", image}...)
	dockerCmd.Stdout = &out
	dockerCmd.Run()
	return out.String() != ""
}

// ECR Regex: https://regex101.com/r/GRxU06/1
var reECR = regexp.MustCompile(`(?P<account>[0-9]+)\.dkr\.ecr\.(?P<region>[a-z0-9\-]+)\.amazonaws\.com`)

func (docker *dockerConfig) refreshImage(image string) {
	app := docker.tgf
	app.Refresh = true // Setting this to true will ensure that dependant built images will also be refreshed

	if app.UseLocalImage {
		app.Debug("Not refreshing %v because `local-image` is set\n", image)
		return
	}

	app.Debug("Checking if there is a newer version of docker image %v\n", image)
	err := getDockerUpdateCmd(image).Run()
	if err != nil {
		matches, _ := reutils.MultiMatch(image, reECR)
		account, accountOk := matches["account"]
		region, regionOk := matches["region"]
		if accountOk && regionOk && docker.awsConfigExist() {
			app.Debug("Failed to pull %v. It is an ECR image, trying again after a login.\n", image)
			loginToECR(account, region)
			must(getDockerUpdateCmd(image).Run())
		} else {
			panic(err)
		}
	}
	touchImageRefresh(image)
	ErrPrintln()
}

func loginToECR(account string, region string) {
	awsSession := session.Must(session.NewSessionWithOptions(session.Options{SharedConfigState: session.SharedConfigEnable}))
	svc := ecr.New(awsSession, &aws.Config{Region: aws.String(region)})
	requestInput := &ecr.GetAuthorizationTokenInput{RegistryIds: []*string{aws.String(account)}}
	result := must(svc.GetAuthorizationToken(requestInput)).(*ecr.GetAuthorizationTokenOutput)

	decodedLogin := string(must(base64.StdEncoding.DecodeString(*result.AuthorizationData[0].AuthorizationToken)).([]byte))
	dockerUpdateCmd := exec.Command("docker", "login", "-u", strings.Split(decodedLogin, ":")[0],
		"-p", strings.Split(decodedLogin, ":")[1], *result.AuthorizationData[0].ProxyEndpoint)
	must(dockerUpdateCmd.Run())
}

func getDockerUpdateCmd(image string) *exec.Cmd {
	dockerUpdateCmd := exec.Command("docker", "pull", image)
	dockerUpdateCmd.Stdout, dockerUpdateCmd.Stderr = os.Stderr, os.Stderr
	return dockerUpdateCmd
}

func getEnviron(noHome bool) (result []string) {
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
