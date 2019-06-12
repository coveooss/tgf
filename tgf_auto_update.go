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
		log.Fatal(err)
	}

	currentDir := filepath.Dir(executablePath)

	os.Setenv("TGF_PATH", currentDir)
	os.Setenv("TGF_LOCAL_VERSION", version)

	updateScript, fetchErr := fetchUpdateScript()
	if fetchErr != nil {
		Println("Fetching update script: ", fetchErr)
	}

	output, errc := exec.Command("bash", "-c", updateScript).Output()

	if errc != nil {
		Println("Error running update script: ", errc.Error())
		Println(string(output))
	}

	return strings.Contains(string(output), "Installing latest tgf")
}

func fetchUpdateScript() (string, error) {
	resp, err := http.Get("https://raw.githubusercontent.com/coveo/tgf/master/get-latest-tgf.sh")
	if err != nil {
		Print("Error fetching update script", err)
	}

	textResponse, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		Print("Error fetching reading request body", err)
	}

	updateScript := string(textResponse)

	return updateScript, err
}
