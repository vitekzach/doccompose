package docker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os/exec"
)

type Client struct {
	bin string // "docker" or "podman"
}

func NewClient(podmanMode bool) Client {
	if podmanMode {
		return Client{bin: "podman"}
	}
	return Client{bin: "docker"}
}

type containerInfo struct {
	Service string `json:"Service"`
	State   string `json:"State"`
}

// GetStatuses returns a map of service name -> is running.
// Returns an empty map (not an error) when the runtime is unavailable or no containers exist.
func (c Client) GetStatuses(composePath string) (map[string]bool, error) {
	out, err := exec.Command(c.bin, "compose", "-f", composePath, "ps", "--format", "json").Output()
	if err != nil {
		return map[string]bool{}, nil
	}

	out = bytes.TrimSpace(out)
	if len(out) == 0 {
		return map[string]bool{}, nil
	}

	containers, err := parseContainerList(out)
	if err != nil {
		return nil, fmt.Errorf("parsing compose ps output: %w", err)
	}

	statuses := make(map[string]bool, len(containers))
	for _, ct := range containers {
		statuses[ct.Service] = ct.State == "running"
	}
	return statuses, nil
}

// parseContainerList handles two formats emitted by different compose versions:
//   - JSON array:   [{...}, {...}]
//   - JSON lines:   {...}\n{...}\n
func parseContainerList(data []byte) ([]containerInfo, error) {
	data = bytes.TrimSpace(data)
	if len(data) == 0 {
		return nil, nil
	}

	if data[0] == '[' {
		var list []containerInfo
		if err := json.Unmarshal(data, &list); err != nil {
			return nil, err
		}
		return list, nil
	}

	var list []containerInfo
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 {
			continue
		}
		var ct containerInfo
		if err := json.Unmarshal(line, &ct); err != nil {
			return nil, err
		}
		list = append(list, ct)
	}
	return list, scanner.Err()
}

// Start runs `<bin> compose up -d <service>`.
func (c Client) Start(composePath, service string) error {
	cmd := exec.Command(c.bin, "compose", "-f", composePath, "up", "-d", service)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", bytes.TrimSpace(out))
	}
	return nil
}

// Stop runs `<bin> compose stop <service>`.
func (c Client) Stop(composePath, service string) error {
	cmd := exec.Command(c.bin, "compose", "-f", composePath, "stop", service)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%s", bytes.TrimSpace(out))
	}
	return nil
}
