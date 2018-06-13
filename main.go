package main

import (
	"bytes"
	"context"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/fatih/color"
	"gopkg.in/alecthomas/kingpin.v2"
)

// Version is initialized at build time through -ldflags "-X main.Version=<version number>"
var version = "master"

var description = `
DESCRIPTION:
TGF ({{ .terragrunt }}) is a Docker frontend for terragrunt/terraform. It automatically maps your current folder,
your HOME folder, your TEMP folder as well of most environment variables to the docker process. You can add -D to
your command to get the exact docker command that is generated.

It then looks in your current folder and all its parents to find a file named '{{ .config }}' to retrieve the
default configuration. If not all configurable values are satisfied and you have an AWS configuration, it will
then try to retrieve the missing elements from the AWS Parameter Store under the key '{{ .parameterStoreKey }}'.

Configurable values are: {{ .options }}.

You can get the full documentation at {{ .readme }} and check for new version at {{ .latest }}.

Any docker image could be used, but TGF specialized images could be found at: {{ .tgfImages }}.

Terragrunt documentation could be found at {{ .terragruntCoveo }} (Coveo fork) or {{ .terragruntGW }} (Gruntwork.io original)

Terraform documentation could be found at {{ .terraform }}.

IMPORTANT:
Most of the tgf command line arguments are in uppercase to avoid potential conflict with the underlying command.
If any of the tgf arguments conflicts with an argument of the desired entry point, you must place that argument
after -- to ensure that they are not interpreted by tgf and are passed to the entry point. Any non conflicting
argument will be passed to the entry point wherever it is located on the invocation arguments.

	tgf ls -- -D   # Avoid -D to be interpretated by tgf as --debug-docker

VERSION: {{ .version }}

AUTHOR:	Coveo
`

var (
	config        = InitConfig()
	dockerOptions []string
	debug         bool
	flushCache    bool
	getImageName  bool
	noHome        bool
	noTemp        bool
	refresh       bool
	mountPoint    string
)

const tgfArgs = "TGF_ARGS"

func trace(args ...interface{}) {
	fmt.Println(time.Now().Format("15:04:05.999999"), fmt.Sprint(args...))
}

func test() {
	trace("Start")

	// Create the context and the client
	ctx := context.Background()
	cli, err := client.NewEnvClient()
	if err != nil {
		panic(err)
	}
	trace("Client & context created")

	// Find image
	image := "coveo/tgf:1.26.1-aws"
	filters := filters.NewArgs()
	filters.Add("reference", image)
	images, err := cli.ImageList(ctx, types.ImageListOptions{Filters: filters})
	if err != nil {
		panic(err)
	}
	if len(images) != 1 {
		fmt.Println("Loading missing image image")
		reader, err := cli.ImagePull(ctx, image, types.ImagePullOptions{})
		if err != nil {
			panic(err)
		}
		defer reader.Close()
		termFd, isTerm := term.GetFdInfo(os.Stderr)
		jsonmessage.DisplayJSONMessagesStream(reader, os.Stderr, termFd, isTerm, nil)
		images, _ = cli.ImageList(ctx, types.ImageListOptions{Filters: filters})
	}
	alpine := images[0]
	trace("Image found", alpine.ID, alpine.Size)

	// Print environment from image
	inspect, _, err := cli.ImageInspectWithRaw(ctx, alpine.ID)
	if err != nil {
		panic(err)
	}
	trace("Inspect image", inspect.ContainerConfig.Env)

	// Create container
	resp, err := cli.ContainerCreate(ctx, &container.Config{
		Image: image,
		Cmd:   []string{"bash", "-c", "for i in {1..15}; do echo $i; sleep 1; done"},
		Tty:   true,
	}, nil, nil, "")
	if err != nil {
		panic(err)
	}
	trace("Container created: ", resp.ID)

	// Start container
	if err := cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{}); err != nil {
		panic(err)
	}

	out, err := cli.ContainerLogs(ctx, resp.ID, types.ContainerLogsOptions{ShowStdout: true, Follow: true})
	if err != nil {
		panic(err)
	}

	defer func() {
		trace("Removing the container")
		err = cli.ContainerRemove(ctx, resp.ID, types.ContainerRemoveOptions{})
		if err != nil {
			panic(err)
		}
		out.Close()
	}()

	go func() {
		io.Copy(os.Stdout, out)
	}()

	// Exec command into a running container
	exec, err := cli.ContainerExecCreate(ctx, resp.ID, types.ExecConfig{
		Cmd:          []string{"bash", "-c", "for i in {0..5}; do echo from exec: $i; sleep 2; done"},
		Tty:          true,
		AttachStdout: true,
		AttachStderr: true,
	})
	if err != nil {
		panic(err)
	}
	trace("Exec created: ", exec.ID)

	hjr, err := cli.ContainerExecAttach(ctx, exec.ID, types.ExecStartCheck{})
	if err != nil {
		panic(err)
	}

	io.Copy(os.Stdout, hjr.Reader)
	defer func() {
		hjr.Close()
	}()

	statusCh, errCh := cli.ContainerWait(ctx, resp.ID, container.WaitConditionNotRunning)
	select {
	case err := <-errCh:
		if err != nil {
			panic(err)
		}
	case <-statusCh:
	}
	trace("Container no longer running")

	trace("List of the containers")
	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
	if err != nil {
		panic(err)
	}
	for _, container := range containers {
		fmt.Printf("%s %s\n", container.ID[:10], container.Image)
	}
}

func main() {
	test()
	os.Exit(0)

	// Handle eventual panic message
	defer func() {
		if err := recover(); err != nil {
			fmt.Fprintln(os.Stderr, errorString("%[1]v (%[1]T)", err))
			os.Exit(1)
		}
	}()

	const gitSource = "https://github.com/coveo/tgf"
	var descriptionBuffer bytes.Buffer
	descriptionTemplate, _ := template.New("usage").Parse(description)
	link := color.New(color.FgHiBlue, color.Italic).SprintfFunc()
	bold := color.New(color.Bold).SprintfFunc()

	descriptionTemplate.Execute(&descriptionBuffer, map[string]interface{}{
		"parameterStoreKey": parameterFolder,
		"config":            configFile,
		"options": color.GreenString(strings.Join([]string{
			dockerImage, dockerImageVersion, dockerImageTag, dockerImageBuild, dockerImageBuildFolder, dockerImageBuildTag,
			dockerRefresh, dockerOptionsTag, recommendedImageVersion, requiredImageVersion, loggingLevel, entryPoint, tgfVersion,
			environment, runBefore, runAfter,
		}, ", ")),
		"readme":          link(gitSource + "/blob/master/README.md"),
		"latest":          link(gitSource + "/releases/latest"),
		"terragruntCoveo": link("https://github.com/coveo/terragrunt/blob/master/README.md"),
		"terragruntGW":    link("https://github.com/gruntwork-io/terragrunt/blob/master/README.md"),
		"terraform":       link("https://www.terraform.io/docs/index.html"),
		"tgfImages":       link("https://hub.docker.com/r/coveo/tgf/tags"),
		"terragrunt":      bold("t") + "erra" + bold("g") + "runt " + bold("f") + "rontend",
		"version":         version,
	})

	var app = NewApplication(kingpin.New(os.Args[0], descriptionBuffer.String()))
	app.Author("Coveo")
	app.HelpFlag = app.HelpFlag.Hidden()
	app.HelpFlag = app.Switch("tgf-help", "Show context-sensitive help (also try --help-man).", 'H')
	app.HelpFlag.Bool()
	kingpin.CommandLine = app.Application

	app.Switch("debug-docker", "Print the docker command issued", 'D').BoolVar(&debug)
	app.Switch("flush-cache", "Invoke terragrunt with --terragrunt-update-source to flush the cache", 'F').BoolVar(&flushCache)
	app.Switch("refresh-image", "Force a refresh of the docker image (alias --ri)").BoolVar(&refresh)
	app.Switch("get-image-name", "Just return the resulting image name (alias --gi)").BoolVar(&getImageName)
	app.Switch("no-home", "Disable the mapping of the home directory (alias --nh)").BoolVar(&noHome)
	app.Switch("no-temp", "Disable the mapping of the temp directory (alias --nt)").BoolVar(&noTemp)
	app.Argument("mount-point", "Specify a mount point for the current folder --mp)").StringVar(&mountPoint)
	app.Argument("docker-arg", "Supply extra argument to Docker (alias --da)").PlaceHolder("<opt>").StringsVar(&dockerOptions)

	var (
		getAllVersions    = app.Switch("all-versions", "Get versions of TGF & all others underlying utilities (alias --av)").Bool()
		getCurrentVersion = app.Switch("current-version", "Get current version infomation (alias --cv)").Bool()
		defaultEntryPoint = app.Argument("entrypoint", "Override the entry point for docker", 'E').PlaceHolder("terragrunt").String()
		image             = app.Argument("image", "Use the specified image instead of the default one").PlaceHolder("coveo/tgf").String()
		imageVersion      = app.Argument("image-version", "Use a different version of docker image instead of the default one (alias --iv)").PlaceHolder("version").Default("-").String()
		imageTag          = app.Argument("tag", "Use a different tag of docker image instead of the default one", 'T').PlaceHolder("latest").Default("-").String()
		awsProfile        = app.Argument("profile", "Set the AWS profile configuration to use", 'P').Default("").String()
		loggingLevel      = app.Argument("logging-level", "Set the logging level (critical=0, error=1, warning=2, notice=3, info=4, debug=5, full=6)", 'L').PlaceHolder("<level>").String()
	)

	app.Switch("ri", "alias for refresh-image)").Hidden().BoolVar(&refresh)
	app.Switch("gi", "alias for get-image-name").Hidden().BoolVar(&getImageName)
	app.Switch("nh", "alias for no-home").Hidden().BoolVar(&noHome)
	app.Switch("nt", "alias for no-temp").Hidden().BoolVar(&noTemp)
	app.Switch("cv", "alias for current-version").Hidden().BoolVar(getCurrentVersion)
	app.Switch("av", "alias for all-versions").Hidden().BoolVar(getAllVersions)
	app.Argument("da", "alias for docker-arg").Hidden().StringsVar(&dockerOptions)
	app.Argument("iv", "alias for image-version").Default("-").Hidden().StringVar(imageVersion)
	app.Argument("mp", "alias for mount-point").Hidden().StringVar(&mountPoint)

	// Split up the managed parameters from the unmanaged ones
	if extraArgs, ok := os.LookupEnv(tgfArgs); ok {
		os.Args = append(os.Args, strings.Split(extraArgs, " ")...)
	}
	managed, unmanaged := app.SplitManaged(os.Args)
	Must(app.Parse(managed))

	// If AWS profile is supplied, we freeze the current session
	if *awsProfile != "" {
		Must(config.InitAWS(*awsProfile))
	}

	if *image != "" {
		config.SetValue(dockerImage, *image)
	}

	if *imageVersion != "-" {
		config.SetValue(dockerImageVersion, *imageVersion)
	}

	if *imageTag != "-" {
		config.SetValue(dockerImageTag, *imageTag)
	}
	config.SetValue(entryPoint, *defaultEntryPoint)
	config.SetDefaultValues()

	var fatalError bool
	for _, err := range config.Validate() {
		switch err := err.(type) {
		case ConfigWarning:
			fmt.Fprintln(os.Stderr, warningString("%v", err))
		case VersionMistmatchError:
			fmt.Fprintln(os.Stderr, errorString("%v", err))
			if *imageVersion == "-" {
				// We consider this as a fatal error only if the version has not been explicitly specified on the command line
				fatalError = true
			}
		default:
			fmt.Fprintln(os.Stderr, errorString("%v", err))
			fatalError = true
		}
	}
	if fatalError {
		os.Exit(1)
	}

	if *getCurrentVersion {
		fmt.Printf("tgf v%s\n", version)
		os.Exit(0)
	}

	if *getAllVersions {
		if filepath.Base(config.EntryPoint) != "terragrunt" {
			fmt.Fprintln(os.Stderr, errorString("--all-version works only with terragrunt as the entrypoint"))
			os.Exit(1)
		}
		fmt.Println("TGF version", version)
		unmanaged = []string{"get-versions"}
	}

	if config.ImageVersion == nil && lastRefresh(config.GetImageName()) > config.Refresh || !checkImage(config.GetImageName()) || refresh {
		refreshImage(config.GetImageName())
	}

	if *loggingLevel != "" {
		config.LogLevel = *loggingLevel
	}

	if config.EntryPoint == "terragrunt" && unmanaged == nil && !debug && !getImageName {
		title := color.New(color.FgYellow, color.Underline).SprintFunc()
		fmt.Println(title("\nTGF Usage\n"))
		app.Usage(nil)
	}

	os.Exit(callDocker(unmanaged...))
}

var warningString = color.New(color.FgYellow).SprintfFunc()
var errorString = color.New(color.FgRed).SprintfFunc()
