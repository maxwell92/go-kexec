package docker

import (
	"github.com/xuant/go-kexec/docker"
	"log"
	"testing"
)

func TestBuildFunction(t *testing.T) {
	config := &docker.DockerConfig{
		HttpHeaders: map[string]string{"User-Agent": "engine-api-cli-1.0"},
		Host:        "unix:///var/run/docker.sock",
		Version:     "v1.22",
		HttpClient:  nil,
	}

	d, err := docker.NewDocker(config)

	if err != nil {
		log.Printf("Failed to NewDocker. Error: %s", err)
		return
	}

	d.BuildFunction("xuant", "faas:v1", "python27")

}
