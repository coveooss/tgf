package main

import (
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RunUpdate runs the update on the current tgf executable
func RunUpdate() bool {
	executablePath, err := os.Executable()
	if err != nil {
		printWarning("Error getting executable path", err)
	}

	currentDir := filepath.Dir(executablePath)

	os.Setenv("TGF_PATH", currentDir)
	os.Setenv("TGF_LOCAL_VERSION", version)

	updateScript, fetchErr := fetchUpdateScript()
	if fetchErr != nil {
		printWarning("Error fetching update script: ", fetchErr)
	}

	output, errc := exec.Command("bash", "-c", updateScript).Output()

	if errc != nil {
		log.Fatal("Error running update script: ", errc.Error())
		log.Fatal(string(output))
	}

	return strings.Contains(string(output), "Installing latest tgf")
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

// ForwardCommand calls tgf with the provided arguments on Unix
func ForwardCommand() {
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Run()
}
