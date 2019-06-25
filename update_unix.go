package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
)

// RunUpdate runs the update on the current tgf executable
func RunUpdate() bool {
	cloneExecutableAt(filepath.Join(getTgfHomeDirectory(), "tgf"))

	os.Setenv("TGF_PATH", getTgfHomeDirectory())

	updateScript, err := fetchUpdateScript()
	if err != nil {
		printWarning("Error fetching update script: ", err)
	}

	output, err := exec.Command("bash", "-c", updateScript).CombinedOutput()
	if err != nil {
		printWarning("Error running update script: ", err)
		Println(string(output))
	}

	return err == nil
}

func fetchUpdateScript() (string, error) {
	resp, err := http.Get("https://raw.githubusercontent.com/coveo/tgf/master/get-latest-tgf.sh")
	if err != nil {
		printWarning("Error fetching update script", err)
	}

	textResponse, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		printWarning("Error reading request body", err)
	}

	updateScript := string(textResponse)

	return updateScript, err
}

// ReRunCopy calls tgf with the provided arguments on Unix
func ReRunCopy() {
	homeExecutablePath := getHomeFileName("tgf")
	currentExecutablePath := getCurrentExecutablePath()

	newArgs := append(os.Args[1:], "--copy-executable", currentExecutablePath)

	cmd := exec.Command(homeExecutablePath, newArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Run()
	cmd.Process.Release()
}

func cloneExecutableAt(newExecutablePath string) string {
	createPathDir(newExecutablePath)
	currentExecutableContent := getCurrentExecutableContent()
	ioutil.WriteFile(newExecutablePath, currentExecutableContent, 755)
	return newExecutablePath
}

func getCurrentExecutablePath() string {
	executablePath, err := os.Executable()
	if err != nil {
		printWarning("Error getting executable path", err)
	}

	return executablePath
}

func getExecutableContent(executablePath string) []byte {
	executableContent, err := ioutil.ReadFile(executablePath)
	if err != nil {
		printWarning("Error reading executable", err)
	}

	return executableContent
}

func getCurrentExecutableContent() []byte {
	return getExecutableContent(getCurrentExecutablePath())
}

func getTgfHomeDirectory() string {
	return path.Dir(getHomeFileName("tgf"))
}
