package docker

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
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
// Always returns the full combined output for display in the log panel.
func (c Client) Start(composePath, service string) (string, error) {
	cmd := exec.Command(c.bin, "compose", "-f", composePath, "up", "-d", service)
	out, err := cmd.CombinedOutput()
	output := string(bytes.TrimSpace(out))
	if err != nil {
		return output, fmt.Errorf("exit error: %w", err)
	}
	return output, nil
}

// Stop runs `<bin> compose stop <service>`.
// Always returns the full combined output for display in the log panel.
func (c Client) Stop(composePath, service string) (string, error) {
	cmd := exec.Command(c.bin, "compose", "-f", composePath, "stop", service)
	out, err := cmd.CombinedOutput()
	output := string(bytes.TrimSpace(out))
	if err != nil {
		return output, fmt.Errorf("exit error: %w", err)
	}
	return output, nil
}

// Down runs `<bin> compose down` to stop and remove all containers.
// Always returns the full combined output for display in the log panel.
func (c Client) Down(composePath string) (string, error) {
	cmd := exec.Command(c.bin, "compose", "-f", composePath, "down")
	out, err := cmd.CombinedOutput()
	output := string(bytes.TrimSpace(out))
	if err != nil {
		return output, fmt.Errorf("exit error: %w", err)
	}
	return output, nil
}

// FollowLogs starts `<bin> compose logs --follow` and streams lines into the
// returned channel. The channel is closed when the process exits. Killing the
// process does not stop the containers because they were started detached.
func (c Client) FollowLogs(composePath string) (<-chan string, error) {
	pr, pw, err := os.Pipe()
	if err != nil {
		return nil, err
	}

	cmd := exec.Command(c.bin, "compose", "-f", composePath, "logs", "--follow", "--timestamps", "--tail", "0")
	cmd.Stdout = pw
	cmd.Stderr = pw

	if err := cmd.Start(); err != nil {
		pr.Close()
		pw.Close()
		return nil, err
	}
	pw.Close() // parent only reads

	ch := make(chan string, 256)
	go func() {
		defer close(ch)
		defer pr.Close()
		scanner := bufio.NewScanner(pr)
		scanner.Buffer(make([]byte, 64*1024), 1024*1024)
		for scanner.Scan() {
			ch <- scanner.Text()
		}
		cmd.Wait()
	}()

	return ch, nil
}

// UpAll runs `<bin> compose up -d` for all services in a single call,
// avoiding network-creation races that occur when services are started in parallel.
func (c Client) UpAll(composePath string) (string, error) {
	cmd := exec.Command(c.bin, "compose", "-f", composePath, "up", "-d")
	out, err := cmd.CombinedOutput()
	output := string(bytes.TrimSpace(out))
	if err != nil {
		return output, fmt.Errorf("exit error: %w", err)
	}
	return output, nil
}
