package main

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/xuant/go-kexec/docker"
	"github.com/xuant/go-kexec/html"
)

var (
	defaultDockerHost    = "unix:///var/run/docker.sock"
	defaultDockerVersion = "v1.22"
	defaultDockerHeaders = map[string]string{"User-Agent": "engine-api-cli-1.0"}

	router = mux.NewRouter()
)

var cookieHandler = securecookie.New(
	securecookie.GenerateRandomKey(64),
	securecookie.GenerateRandomKey(32))

func main() {
	router.HandleFunc("/", indexPageHandler)
	router.HandleFunc("/create", createFunctionHandler)
	router.HandleFunc("/internal", internalPageHandler)
	router.HandleFunc("/login", loginHandler).Methods("POST")
	router.HandleFunc("/logout", logoutHandler).Methods("POST")

	staticServer := http.StripPrefix("/ui/", http.FileServer(http.Dir("./ui")))
	router.PathPrefix("/ui").Handler(staticServer)

	http.Handle("/", router)
	http.ListenAndServe(":8080", nil)
}

func createFunctionHandler(w http.ResponseWriter, r *http.Request) {
	code := r.FormValue("codeTextarea")
	log.Printf("Code uploaded:\n%s", code)
	if code == "" {
		http.Redirect(response, request, "/internal", 302)
		return
	}

	exeFile, err := os.Create(filepath.Join(docker.IBContext, docker.ExecutionFile))

	if err != nil {
		fmt.Fprintf(w, "Permission denied. Unable to create execution file %s", filepath.Join(docker.IBContext, docker.ExecutionFile))
		return
	}
	defer exeFile.Close()

	if _, err = exeFile.WriteString(code); err != nil {
		fmt.Fprintln(w, err)
		return
	}

	fmt.Fprintln(w, "Function created.")
	return
}

func indexPageHandler(response http.ResponseWriter, request *http.Request) {
	fmt.Fprintf(response, html.IndexPage)
}

func loginHandler(response http.ResponseWriter, request *http.Request) {
	name := request.FormValue("name")
	pass := request.FormValue("password")
	redirectTarget := "/"
	if name != "" && pass != "" {
		// ... check credentials
		setSession(name, response)
		redirectTarget = "/internal"
	}
	http.Redirect(response, request, redirectTarget, 302)
}

func internalPageHandler(response http.ResponseWriter, request *http.Request) {
	userName := getUserName(request)
	if userName != "" {
		fmt.Fprintf(response, html.Upload, userName)
	} else {
		http.Redirect(response, request, "/", 302)
	}
}

func logoutHandler(response http.ResponseWriter, request *http.Request) {
	clearSession(response)
	http.Redirect(response, request, "/", 302)
}

func setSession(userName string, response http.ResponseWriter) {
	value := map[string]string{
		"name": userName,
	}
	if encoded, err := cookieHandler.Encode("session", value); err == nil {
		cookie := &http.Cookie{
			Name:  "session",
			Value: encoded,
			Path:  "/",
		}
		http.SetCookie(response, cookie)
	}
}

func clearSession(response http.ResponseWriter) {
	cookie := &http.Cookie{
		Name:   "session",
		Value:  "",
		Path:   "/",
		MaxAge: -1,
	}
	http.SetCookie(response, cookie)
}

func getUserName(request *http.Request) (userName string) {
	if cookie, err := request.Cookie("session"); err == nil {
		cookieValue := make(map[string]string)
		if err = cookieHandler.Decode("session", cookie.Value, &cookieValue); err == nil {
			userName = cookieValue["name"]
		}
	}
	return userName
}
