package kexec

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
	"golang.org/x/net/context"
)

const (
	ibContext     = "/tmp/faas-imagebuild-context/"
	executionFile = "exec"

	defaultDockerHost       = "unix:///var/run/docker.sock"
	defaultDockerVersion    = "v1.22"
	defaultDockerHttpClient = nil
	defaultDockerHeaders    = map[string]string{"User-Agent": "engine-api-cli-1.0"}
)

func main() {
	cli, err := client.NewClient(defaultHost, defaultVersion, defaultHttpClient, defaultHeaders)
	if err != nil {
		panic(err)
	}

	options := types.ContainerListOptions{All: true}
	containers, err := cli.ContainerList(context.Background(), options)
	if err != nil {
		panic(err)
	}

	for _, c := range containers {
		fmt.Println(c.ID, c.Names, c.Image)
	}
}

type DockerConfig struct {
	HttpHeaders map[string]string
	Host        string
	Version     string
	HttpClient  *http.Client
}

type Docker struct {
	client *client.Client
}

func NewDocker(c *DockerConfig) (*Docker, error) {
	cli, err := client.NewClient(c.Host, c.Version, c.HttpClient, c.HttpHeaders)
	if err != nil {
		return nil, err
	}
	return &Docker{
		client: cli,
	}
}

func (d *Docker) BuildFunction(namespace, funcName, templateName string) error {
	err := setRuntimeTemplate(templateName)

	if err != nil {
		log.Printf("Failed to set up runtime template. Error: %s", err)
		return err
	}

	ctx, err := os.Open(ibContext)

	if err != nil {
		log.Printf("Failed to open image build context %s. Error: %s", ibContext, err)
		return err
	}

	opts := types.ImageBuildOptions{
		Tags:   []string{"faas:v0.1"},
		Squash: true,
	}

	resp, err := d.client.ImageBuild(context.Background(), ctx, opts)
	defer resp.Body.Close()

	if err != nil {
		log.Printf("Failed to build image. Error: %s", err)
		return err
	}

	buf, err := ioutil.ReadAll(resp.Body)

	if err != nil {
		log.Printf("Failed to read response body. Error: %s", err)
		return err
	}

	log.Printf("Image build response Body: %s", string(buf))
	log.Printf("Image build response OSType: %s", resp.OSType)

	return nil
}

var python27Template = `FROM python:2.7
ADD . ./
ENTRYPOINT [ "python", "exec" ]
`

// setRuntimeEnv creates the runtime environment for building a docker image.
//
// Based on the templateName, this method will create a corresponding Dockerfile
// in ibContext (i.e. /tmp/faas-imagebuild-context). To make the build process fast,
// runtime template should be proloaded onto the system.
//
// Now supporting Python27 only. Other template can be added easi
func setRuntimeTemplate(templateName string) error {
	switch templateName {
	case "python27":
		ioutil.WriteFile(ibContext+"Dockerfile", []byte(python27Template), 0644)
		return nil
	default:
		return errors.New("Runtime template " + templateName + " invalid or not supported yet.")

	}
}
