package main

import (
	"errors"
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
	router        = mux.NewRouter()
	cookieHandler = securecookie.New(
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32))

	d = docker.NewDocker(&docker.DockerConfig{
		HttpHeaders: map[string]string{"User-Agent": "engin-api-cli-1.0"},
		Host:        "unix:///var/run/docker.sock",
		Version:     "v1.22",
		HttpClient:  nil,
	})
)

func main() {
	router.HandleFunc("/", indexPageHandler)
	router.HandleFunc("/create", createFunctionHandler)
	router.HandleFunc("/internal", internalPageHandler)
	router.HandleFunc("/login", loginHandler).Methods("POST")
	router.HandleFunc("/logout", logoutHandler).Methods("POST")

	/* ... Static files not used currently
	staticServer := http.StripPrefix("/ui/", http.FileServer(http.Dir("./ui")))
	router.PathPrefix("/ui").Handler(staticServer)
	*/

	http.Handle("/", router)
	http.ListenAndServe(":8080", nil)
}

func createFunctionHandler(response http.ResponseWriter, request *http.Request) {
	userName := getUserName(request)
	if userName == "" {
		http.Redirect(response, request, "/", http.StatusFound)
	} else {
		// Read function code from the form
		// Before the function can be created, several steps needs to be
		// executed:
		//   1. Check if uploaded function code is empty
		//   2. Create the execution file for the function
		//   3. Write the function code to the execution file
		//   4. Build the function (ie build docker image)
		functionName := request.FormValue("functionName")
		runtime := request.FormValue("runtime")
		code := request.FormValue("codeTextarea")
		if functionName == "" || runtime == "" || code == "" {
			err := errors.New("Something's wrong with FunctionName/Runtime/Code.")
			log.Printf("Function failed: something's wrong with Function Name/Runtime/Code.")
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		log.Printf("Code uploaded:\n%s", code)
		log.Printf("Start creating function \"%s\" with runtime \"%s\"", functionName, runtime)

		exeFileName := filepath.Join(docker.IBContext, docker.ExecutionFile)
		exeFile, err := os.Create(exeFileName)

		if err != nil {
			log.Printf("Function failed: %s", err)
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		defer exeFile.Close()

		if _, err = exeFile.WriteString(code); err != nil {
			log.Printf("Function failed: %s", err)
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}

		if err = d.BuildFunction(userName, functionName, runtime); err != nil {
			log.Printf("Function failed: %s", err)
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}

		// If all the above operation succeeded, the function is created
		// successfully.
		fmt.Fprintf(response, html.FunctionCreatedPage)
	}
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
	http.Redirect(response, request, redirectTarget, http.StatusFound)
}

func internalPageHandler(response http.ResponseWriter, request *http.Request) {
	userName := getUserName(request)
	if userName != "" {
		fmt.Fprintf(response, html.InternalPage, userName)
	} else {
		http.Redirect(response, request, "/", http.StatusFound)
	}
}

func logoutHandler(response http.ResponseWriter, request *http.Request) {
	clearSession(response)
	http.Redirect(response, request, "/", http.StatusFound)
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
