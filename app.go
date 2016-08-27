package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

func uploadHandler(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.RemoteAddr, r.Method, r.URL)

	file, header, err := r.FormFile("file")
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}

	defer file.Close()

	out, err := os.Create(ibContext + executionFile)
	if err != nil {
		fmt.Fprintf(w, "Permission denied. Unable to create file "+IBContext+executionFile)
		return
	}
	defer out.Close()

	_, err = io.Copy(out, file)
	if err != nil {
		fmt.Fprintln(w, err)
	}

	fmt.Fprintf(w, "File uploaded successfully : ")
	fmt.Fprintf(w, header.Filename)
}

func main() {
	http.HandleFunc("/receive", uploadHandler)
	http.ListenAndServe(":8080", nil)
}
