package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/xuant/go-kexec/docker"
)

var (
	d = *Docker
)

func init() {
	cfg := &DockerConfig{
		HttpHeaders: docker.defaultDockerHeaders,
		Host:        docker.defaultDockerHost,
		Version:     docker.defaultDockerVersion,
		HttpClient:  nil,
	}

	d, err := docker.NewDocker(cfg)
	if err != nil {
		log.Printf("Failed to NewDocker. Error: %s", err)
	}
}

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)

	file, header, err := r.FormFile("file")
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	defer file.Close()

	out, err := os.Create(docker.ibContext + docker.executionFile)
	if err != nil {
		fmt.Fprintf(w, "Permission denied. Unable to create file "+docker.ibContext+docker.executionFile)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		fmt.Fprintln(w, err)
	}

	fmt.Fprintf(w, "File uploaded successfully : %s", header.Filename)
}

func main() {
	http.HandleFunc("/receive", uploadHandler)
	http.ListenAndServe(":8080", nil)
}
