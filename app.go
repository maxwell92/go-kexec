package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/xuant/go-kexec/docker"
)

var (
	defaultDockerHost    = "unix:///var/run/docker.sock"
	defaultDockerVersion = "v1.22"
	defaultDockerHeaders = map[string]string{"User-Agent": "engine-api-cli-1.0"}
)

func uploadFile(w http.ResponseWriter, r *http.Request) error {

	code := r.FormValue("codeTextarea")
	log.Printf("Code uploaded:\n%s", code)

	if code == "" {
		log.Println("Code is Empty, do nothing.")
		return fmt.Errorf("Code is empty, do nothing.", nil)
	}

	exeFile, err := os.Create(docker.IBContext + docker.ExecutionFile)

	if err != nil {
		fmt.Fprintf(w, "Permission denied. Unable to create execution file %s", filepath.Join(docker.IBContext, docker.ExecutionFile))
		return err
	}
	defer exeFile.Close()

	if _, err = exeFile.WriteString(code); err != nil {
		fmt.Fprintln(w, err)
		return err
	}

	fmt.Fprintln(w, "Function created.")
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
		if err := d.BuildFunction("xuant", "aceeditor", "python27"); err != nil {
			log.Println(err)
			return
		}
	})

	fs := http.FileServer(http.Dir("/Users/xuan_tang/goproject/src/github.com/xuant/go-kexec/ui"))
	http.Handle("/", fs)

	log.Fatal(http.ListenAndServe(":8080", nil))
}
