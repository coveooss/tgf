package main

import (
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
)

// RunUpdate runs the update on the current tgf executable
func RunUpdate() bool {
	executablePath, err := os.Executable()
	if err != nil {
		printWarning("Error getting executable path", err)
	}

	currentDir := filepath.Dir(executablePath)

	os.Setenv("TGF_PATH", currentDir)

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

// ReRun calls tgf with the provided arguments on Unix
func ReRun() {
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Run()
}
