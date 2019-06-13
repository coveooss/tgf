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

// RunUpdateUnix runs the update on the current tgf executable
func RunUpdateUnix() bool {
	executablePath, err := os.Executable()
	if err != nil {
		log.Fatal(err)
	}

	currentDir := filepath.Dir(executablePath)

	os.Setenv("TGF_PATH", currentDir)
	os.Setenv("TGF_LOCAL_VERSION", version)

	updateScript, fetchErr := fetchUpdateScriptUnix()
	if fetchErr != nil {
		log.Fatal("Fetching update script: ", fetchErr)
	}

	output, errc := exec.Command("bash", "-c", updateScript).Output()

	if errc != nil {
		log.Fatal("Error running update script: ", errc.Error())
		log.Fatal(string(output))
	}

	return strings.Contains(string(output), "Installing latest tgf")
}

func fetchUpdateScriptUnix() (string, error) {
	resp, err := http.Get("https://raw.githubusercontent.com/coveo/tgf/master/get-latest-tgf.sh")
	if err != nil {
		log.Fatal("Error fetching update script", err)
	}

	textResponse, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal("Error fetching reading request body", err)
	}

	updateScript := string(textResponse)

	return updateScript, err
}

// ForwardCommandUnix calls tgf with the provided arguments on Unix
func ForwardCommandUnix() {
	cmd := exec.Command(os.Args[0], os.Args[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	cmd.Run()
}
