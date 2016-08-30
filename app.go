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
	defaultDockerHost    = "unix:///var/run/docker.sock"
	defaultDockerVersion = "v1.22"
	defaultDockerHeaders = map[string]string{"User-Agent": "engine-api-cli-1.0"}
)

/*
func uploadHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
	uploadFile(w, r)
	d.BuildFunction("xuant", "testfunc", "python27")
}
*/
func uploadFile(w http.ResponseWriter, r *http.Request) error {

	file, header, err := r.FormFile("file")

	if err != nil {
		fmt.Fprintln(w, err)
		return err
	}

	defer file.Close()

	out, err := os.Create(docker.IBContext + docker.ExecutionFile)

	if err != nil {
		fmt.Fprintf(w, "Permission denied. Unable to create file "+docker.IBContext+docker.ExecutionFile)
		return err
	}

	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		fmt.Fprintln(w, err)
		return err
	}

	fmt.Fprintf(w, "File uploaded successfully: %s", header.Filename)
	return nil
}

func main() {
	cfg := &docker.DockerConfig{
		HttpHeaders: defaultDockerHeaders,
		Host:        defaultDockerHost,
		Version:     defaultDockerVersion,
		HttpClient:  nil,
	}

	d, err := docker.NewDocker(cfg)
	if err != nil {
		log.Printf("Failed to NewDocker. Error: %s", err)
	}

	http.HandleFunc("/receive", func(w http.ResponseWriter, r *http.Request) {
		log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)
		if err := uploadFile(w, r); err != nil {
			log.Println(err)
			return
		}
		log.Println("File uploaded")
		if err := d.BuildFunction("xuant", "testfunc", "python27"); err != nil {
			log.Println(err)
			return
		}
	})
	log.Fatal(http.ListenAndServe(":8080", nil))
}
