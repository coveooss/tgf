package main

const dockerSocketMountPattern = "/%[1]s:%[1]s"

func getDockerGroup() string {
	return "root"
}
