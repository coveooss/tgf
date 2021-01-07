package main

import "fmt"

const dockerSocketMountPattern = "/%[1]s:%[1]s"

func getDockerMountArgs() []string {
	return []string{"-v", getDockerSocketMount(), "--group-add", getDockerGroup()}
}

func getDockerSocketMount() string {
	return fmt.Sprintf(dockerSocketMountPattern, dockerSocketFile)
}

func getDockerGroup() string {
	return "root"
}
