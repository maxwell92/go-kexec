package main

import (
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gorilla/mux"
	"github.com/gorilla/securecookie"
	"github.com/wayn3h0/go-uuid"
	"github.com/xuant/go-kexec/docker"
	"github.com/xuant/go-kexec/html"
	"github.com/xuant/go-kexec/kexec"
	"gopkg.in/ldap.v2"
)

var argConfigFile = flag.String("config", "", "Config file")

// Error represents a handler error. It provides methods for a HTTP status
// code and embeds the built-in error interface.
type Error interface {
	error
	Status() int
}

// StatusError represents an error with an associated HTTP status code.
type StatusError struct {
	Code int
	Err  error
}

// Allows StatusError to satisfy the error interface.
func (se StatusError) Error() string {
	return se.Err.Error()
}

// Returns our HTTP status code.
func (se StatusError) Status() int {
	return se.Code
}

type appConfig struct {
	DockerRegistry string
	LDAPcfg        ldapConfig
}
type ldapConfig struct {
	LDAPServer  []string
	LDAPPort    int
	LDAPRetries int
	LDAPBaseDn  string
}
type appContext struct {
	d             *docker.Docker
	k             *kexec.Kexec
	cookieHandler *securecookie.SecureCookie
	conf          *appConfig
}
type appHandler struct {
	*appContext
	H func(*appContext, http.ResponseWriter, *http.Request) error
}

func (ah appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := ah.H(ah.appContext, w, r)
	if err != nil {
		switch e := err.(type) {
		case Error:
			// We can retrieve the status here and write out a specific
			// HTTP status code.
			log.Printf("HTTP %d - %s", e.Status(), e)
			http.Error(w, e.Error(), e.Status())
		default:
			// Any error types we don't specifically look out for default
			// to serving a HTTP 500
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
		}
	}
}

func main() {
	flag.Parse()
	configFile, err := ioutil.ReadFile(*argConfigFile)
	if err != nil {
		log.Fatalf("Cannot read config file %s: %v\n", *argConfigFile, err)
	}
	var conf appConfig
	err = json.Unmarshal(configFile, &conf)
	if err != nil {
		log.Fatalf("Cannot load config file %s: %v\n", *argConfigFile, err)
	}

	// cookie handling
	cookieHandler := securecookie.New(
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32),
	)

	// docker handler for creating function and pushing function image
	// to docker registry
	d := docker.NewDocker(
		// http headers
		map[string]string{"User-Agent": "engin-api-cli-1.0"},
		// docker host
		"unix:///var/run/docker.sock",
		// docker api version
		"v1.22",
		// http client
		nil,
	)

	// kubernetes handler for calling function and pulling function
	// execution logs
	k, _ := kexec.NewKexec(&kexec.KexecConfig{
		KubeConfig: os.Getenv("HOME") + "/.kube/config",
	})

	context := &appContext{d: d, k: k, cookieHandler: cookieHandler, conf: &conf}
	// gorilla web http router
	router := mux.NewRouter()
	// IndexPageHandler handles index page (i.e. login page)
	router.Handle("/", appHandler{context, IndexPageHandler})

	// LoginHandler create session from login page information,
	// do basic authentication, and redirect to the internal
	// control panel if authenticated.
	//
	// LogoutHandler clears session and redirect to index page.
	router.Handle("/login", appHandler{context, LoginHandler}).Methods("POST")
	router.Handle("/logout", appHandler{context, LogoutHandler})

	// InternalPageHandler displays the internal control panel
	router.Handle("/internal", appHandler{context, InternalPageHandler})

	// CreateFunctionHandler handles `create function` request
	router.Handle("/create", appHandler{context, CreateFunctionHandler})

	// CallFunctionHandler handles `call function` request
	router.Handle("/call", appHandler{context, CallFunctionHandler})

	/* ... Static files not used currently
	staticServer := http.StripPrefix("/ui/", http.FileServer(http.Dir("./ui")))
	router.PathPrefix("/ui").Handler(staticServer)
	*/

	http.Handle("/", router)
	panic(http.ListenAndServe(":8080", nil))
}

func CallFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {

	userName := getUserName(a, request)
	functionName := getFunctionName(request)

	if userName == "" || functionName == "" {

		// Empty username is not allowed to call function
		http.Redirect(response, request, "/", http.StatusFound)

	} else {

		// create a uuid for each function call. This uuid can be
		// seen as the execution id for the function (notice there
		// are multiple executions for a single function)
		uuid, err := uuid.NewTimeBased()

		if err != nil {

			// Log on server side and notify client
			log.Println("Failed to create uuid for function call.")

			// Return immediately when there is an error
			return StatusError{http.StatusInternalServerError, err}
		}

		uuidStr := uuid.String() // uuidStr needed when fetching log

		jobname := functionName + "-" + uuidStr
		image := a.conf.DockerRegistry + "/" + userName + "/" + functionName
		labels := make(map[string]string)

		if err = a.k.CallFunction(jobname, image, userName, labels); err != nil {

			log.Printf("Failed to call function %s.", functionName)

			return StatusError{http.StatusInternalServerError, err}
		}

		fmt.Fprintf(response, html.FunctionCalledPage)
	}
	return nil
}

func getFunctionName(request *http.Request) string {
	return "Not implemented yet."
}

func CreateFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName == "" {

		// Empty username is not allowed to create function
		http.Redirect(response, request, "/", http.StatusFound)

	} else {

		// Read function code from the form
		// Before the function can be created, several steps needs to be
		// executed.
		//   2. Create the execution file for the function
		//   3. Write the function code to the execution file
		//   4. Build the function (ie build docker image)
		functionName := request.FormValue("functionName")
		runtime := request.FormValue("runtime")
		code := request.FormValue("codeTextarea")

		// Check if function name is empty;
		// check if runtime template is chosen;
		// check if the input code is empty.
		if functionName == "" || runtime == "" || code == "" {
			err := errors.New("Something's wrong with FunctionName/Runtime/Code.")
			return StatusError{http.StatusInternalServerError, err}
		}

		log.Printf("Code uploaded:\n%s", code)
		log.Printf("Start creating function \"%s\" with runtime \"%s\"", functionName, runtime)

		// Create the execution file for the function
		exeFileName := filepath.Join(docker.IBContext, docker.ExecutionFile)
		exeFile, err := os.Create(exeFileName)

		if err != nil {
			return StatusError{http.StatusInternalServerError, err}
		}
		defer exeFile.Close()

		// Write the function into the execution file
		if _, err = exeFile.WriteString(code); err != nil {
			return StatusError{http.StatusInternalServerError, err}
		}

		// Build funtion
		if err = a.d.BuildFunction(a.conf.DockerRegistry, userName, functionName, runtime); err != nil {
			log.Println("Build function failed")
			return StatusError{http.StatusInternalServerError, err}
		}

		// Register function to configured docker registry
		if err = docker.RegisterFunction(a.conf.DockerRegistry, userName, functionName); err != nil {
			log.Println("Register function failed")
			return StatusError{http.StatusInternalServerError, err}
		}

		// If all the above operation succeeded, the function is created
		// successfully.
		fmt.Fprintf(response, html.FunctionCreatedPage)
	}
	return nil
}

func IndexPageHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName != "" {
		//Already logged in, show internal page
		fmt.Fprintf(response, html.InternalPage, userName)
	} else {
		fmt.Fprintf(response, html.IndexPage)
	}
	return nil
}

func LoginHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	name := request.FormValue("name")
	pass := request.FormValue("password")
	redirectTarget := "/"
	if name != "" && pass != "" {
		// ... check credentials
		ok, err := checkCredentials(a, name, pass)
		if !ok {
			errMsg := err.Error()
			// Check if it is a LDAP specific error
			for code, msg := range ldap.LDAPResultCodeMap {
				if ldap.IsErrorWithCode(err, code) {
					errMsg = msg
					break
				}
			}
			fmt.Fprintf(response, "<h1>Login</h1>"+
				"<p>Error: %s</p>"+
				"<form method=\"post\" action=\"/login\">"+
				"<label for=\"name\">User name</label>"+
				"<input type=\"text\" id=\"name\" name=\"name\">"+
				"<label for=\"password\">Password</label>"+
				"<input type=\"password\" id=\"password\" name=\"password\">"+
				"<button type=\"submit\">Login</button>"+
				"</form>", errMsg)
			return nil
		}
		setSession(a, name, response)
		redirectTarget = "/internal"
	}
	http.Redirect(response, request, redirectTarget, http.StatusFound)
	return nil
}

func InternalPageHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName != "" {
		fmt.Fprintf(response, html.InternalPage, userName)
	} else {
		http.Redirect(response, request, "/", http.StatusFound)
	}
	return nil
}

func LogoutHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	clearSession(response)
	log.Println("Logged out")
	http.Redirect(response, request, "/", http.StatusFound)
	return nil
}

func setSession(a *appContext, userName string, response http.ResponseWriter) {
	value := map[string]string{
		"name": userName,
	}
	if encoded, err := a.cookieHandler.Encode("session", value); err == nil {
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

func getUserName(a *appContext, request *http.Request) (userName string) {
	if cookie, err := request.Cookie("session"); err == nil {
		cookieValue := make(map[string]string)
		if err = a.cookieHandler.Decode("session", cookie.Value, &cookieValue); err == nil {
			userName = cookieValue["name"]
		}
	}
	return userName
}

func checkCredentials(a *appContext, name string, pass string) (bool, error) {
	var l *ldap.Conn
	var err error

	servers := a.conf.LDAPcfg.LDAPServer
	port := a.conf.LDAPcfg.LDAPPort
	retries := a.conf.LDAPcfg.LDAPRetries
	username := fmt.Sprintf(a.conf.LDAPcfg.LDAPBaseDn, name)

	log.Println("Authenticating user", name)

	//Connect to LDAP servers with retries
	for i := 0; i < retries; i++ {
		for _, s := range servers {
			log.Println("Connecting to LDAP server", s, "......")
			l, err = ldap.DialTLS("tcp", fmt.Sprintf("%s:%d", s, port),
				&tls.Config{ServerName: s})
			if err == nil {
				break
			}
		}
		if err == nil {
			log.Println("Connected")
			break
		}
	}

	if err != nil {
		log.Println(err)
		return false, err
	}
	defer l.Close()

	//Bind
	err = l.Bind(username, pass)
	if err != nil {
		log.Println(err)
		return false, err
	}
	log.Printf("Bound user %s\n", name)
	return true, nil
}
