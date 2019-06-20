package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"strings"
)

// RunUpdate runs the update on the current tgf executable
func RunUpdate() bool {
	currentExecutablePath, err := os.Executable()
	if err != nil {
		printWarning("Error getting executable path", err)
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		printWarning("Error getting home directory", err)
	}
	homeExecutableDir := path.Join(homeDir, ".tgf")

	currentExecutableContent, err := ioutil.ReadFile(currentExecutablePath)
	ioutil.WriteFile(homeExecutableDir, currentExecutableContent, 755)

	Println("pathss", homeExecutableDir, currentExecutablePath)
	os.Setenv("TGF_PATH", homeExecutableDir)

	updateScript, err := fetchUpdateScript()
	if err != nil {
		printWarning("Error fetching update script: ", err)
	}

	output, err := exec.Command("bash", "-c", updateScript).CombinedOutput()
	if err != nil {
		printWarning("Error running update script: ", err)
	}
	Println(string(output))

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

// ReRun calls tgf with the provided arguments on Unix
func ReRun(pathToSwap string) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		printWarning("Error getting home directory", err)
	}
	homeExecutablePath := path.Join(homeDir, ".tgf", "tgf")

	cmd := exec.Command(homeExecutablePath, strings.Join(os.Args[1:], " "), "--swap", pathToSwap)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// SwapExecutables Swaps the current executable with the one at the provided path
func SwapExecutables(pathToSwap string) bool {
	currentExecutablePath, err := os.Executable()
	if err != nil {
		printWarning("Error getting executable path", err)
	}

	Println("swaping : ", pathToSwap, "with", currentExecutablePath)
	currentExecutableContent, err := ioutil.ReadFile(currentExecutablePath)
	ioutil.WriteFile(pathToSwap, currentExecutableContent, 755)

	return true
}
