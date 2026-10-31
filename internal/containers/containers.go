// Package containers wraps the Docker CLI. It is entirely optional: the UI
// hides this module when Docker is not installed.
package containers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// ErrUnavailable is returned when Docker is not installed or not running.
var ErrUnavailable = errors.New("Docker is not installed or not running. Install Docker Desktop / docker engine, then restart DevVault")

type Service struct{}

func New() *Service { return &Service{} }

func (s *Service) docker(ctx context.Context, args ...string) (string, error) {
	c, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(c, "docker", args...)
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(errOut.String())
		if msg == "" {
			msg = err.Error()
		}
		if isNotFound(err) {
			return "", ErrUnavailable
		}
		return "", errors.New(msg)
	}
	return out.String(), nil
}

func isNotFound(err error) bool {
	if ee, ok := err.(*exec.Error); ok {
		return errors.Is(ee, exec.ErrNotFound) || ee.Err == exec.ErrNotFound
	}
	return false
}

// Detect reports Docker availability and server version.
func (s *Service) Detect() (bool, string) {
	v, err := s.docker(context.Background(), "version", "--format", "{{.Server.Version}}")
	if err != nil {
		return false, ""
	}
	return true, strings.TrimSpace(v)
}

// Container is one container row.
type Container struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Image  string `json:"image"`
	State  string `json:"state"`
	Status string `json:"status"`
	Ports  string `json:"ports"`
}

// List returns containers (all states).
func (s *Service) List() ([]Container, error) {
	out, err := s.docker(context.Background(), "ps", "-a", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	cs := []Container{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var raw map[string]string
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		cs = append(cs, Container{
			ID: raw["ID"], Name: raw["Names"], Image: raw["Image"],
			State: raw["State"], Status: raw["Status"], Ports: raw["Ports"],
		})
	}
	return cs, nil
}

// Image is one docker image row.
type Image struct {
	ID        string `json:"id"`
	Repo      string `json:"repo"`
	Tag       string `json:"tag"`
	Size      string `json:"size"`
	CreatedAt string `json:"createdAt"`
}

// Images lists docker images.
func (s *Service) Images() ([]Image, error) {
	out, err := s.docker(context.Background(), "images", "--format", "{{json .}}")
	if err != nil {
		return nil, err
	}
	is := []Image{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		var raw map[string]string
		if err := json.Unmarshal([]byte(line), &raw); err != nil {
			continue
		}
		is = append(is, Image{ID: raw["ID"], Repo: raw["Repository"], Tag: raw["Tag"], Size: raw["Size"], CreatedAt: raw["CreatedAt"]})
	}
	return is, nil
}

// Start starts a container.
func (s *Service) Start(id string) error {
	_, err := s.docker(context.Background(), "start", sanitizeID(id))
	return err
}

// Stop stops a container.
func (s *Service) Stop(id string) error {
	_, err := s.docker(context.Background(), "stop", sanitizeID(id))
	return err
}

// Logs returns the last N lines of container logs.
func (s *Service) Logs(id string, tail int) (string, error) {
	if tail <= 0 || tail > 2000 {
		tail = 200
	}
	return s.docker(context.Background(), "logs", "--tail", itoa(tail), sanitizeID(id))
}

// sanitizeID ensures the id cannot inject extra docker flags.
func sanitizeID(id string) string {
	id = strings.TrimSpace(id)
	if id == "" || strings.HasPrefix(id, "-") || strings.ContainsAny(id, " \t\n;") {
		return ""
	}
	return id
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b [20]byte
	i := len(b)
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
