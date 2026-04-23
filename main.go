package main

import (
	"flag"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/vitekzach/doccompose/compose"
	"github.com/vitekzach/doccompose/docker"
	"github.com/vitekzach/doccompose/ui"
)

var candidatePaths = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

func findComposeFile() (string, error) {
	for _, path := range candidatePaths {
		if _, err := os.Stat(path); err == nil {
			return path, nil
		}
	}
	return "", fmt.Errorf(
		"no compose file found in current directory\nlooked for: compose.yaml, compose.yml, docker-compose.yaml, docker-compose.yml",
	)
}

func main() {
	podmanMode := flag.Bool("podmanmode", false, "use podman instead of docker")
	flag.Parse()

	path, err := findComposeFile()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	composeFile, err := compose.ParseFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error parsing compose file:", err)
		os.Exit(1)
	}

	client := docker.NewClient(*podmanMode)

	p := tea.NewProgram(ui.New(path, composeFile, client), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
