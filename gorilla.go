package main

import (
	"crypto/tls"
	"errors"
	"fmt"
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

var (
	// default docker registry
	defaultDockerRegistry = "registry.paas.symcpe.com:443"

	// gorilla web http router
	router = mux.NewRouter()

	// cookie handling
	cookieHandler = securecookie.New(
		securecookie.GenerateRandomKey(64),
		securecookie.GenerateRandomKey(32),
	)

	// docker handler for creating function and pushing function image
	// to docker registry
	d = docker.NewDocker(
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
	k, _ = kexec.NewKexec(&kexec.KexecConfig{
		KubeConfig: os.Getenv("HOME") + "/.kube/config",
	})
)

func main() {

	// IndexPageHandler handles index page (i.e. login page)
	router.HandleFunc("/", IndexPageHandler)

	// LoginHandler create session from login page information,
	// do basic authentication, and redirect to the internal
	// control panel if authenticated.
	//
	// LogoutHandler clears session and redirect to index page.
	router.HandleFunc("/login", LoginHandler).Methods("POST")
	router.HandleFunc("/logout", LogoutHandler)

	// InternalPageHandler displays the internal control panel
	router.HandleFunc("/internal", InternalPageHandler)

	// CreateFunctionHandler handles `create function` request
	router.HandleFunc("/create", CreateFunctionHandler)

	// CallFunctionHandler handles `call function` request
	router.HandleFunc("/call", CallFunctionHandler)

	/* ... Static files not used currently
	staticServer := http.StripPrefix("/ui/", http.FileServer(http.Dir("./ui")))
	router.PathPrefix("/ui").Handler(staticServer)
	*/

	http.Handle("/", router)
	panic(http.ListenAndServe(":8080", nil))
}

func CallFunctionHandler(response http.ResponseWriter, request *http.Request) {

	userName := getUserName(request)
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
			log.Printf("Failed to create uuid for function call. Error: %s", err)
			http.Error(response, err.Error(), http.StatusInternalServerError)

			// Return immediately when there is an error
			return
		}

		uuidStr := uuid.String() // uuidStr needed when fetching log

		jobname := functionName + "-" + uuidStr
		image := defaultDockerRegistry + "/" + userName + "/" + functionName
		labels := make(map[string]string)

		if err = k.CallFunction(jobname, image, userName, labels); err != nil {

			log.Printf("Failed to call function %s. Error: %s", functionName, err)
			http.Error(response, err.Error(), http.StatusInternalServerError)

			return
		}

		fmt.Fprintf(response, html.FunctionCalledPage)
	}
}

func getFunctionName(request *http.Request) string {
	return "Not implemented yet."
}

func CreateFunctionHandler(response http.ResponseWriter, request *http.Request) {
	userName := getUserName(request)
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
			log.Printf("Function failed: something's wrong with Function Name/Runtime/Code.")
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}

		log.Printf("Code uploaded:\n%s", code)
		log.Printf("Start creating function \"%s\" with runtime \"%s\"", functionName, runtime)

		// Create the execution file for the function
		exeFileName := filepath.Join(docker.IBContext, docker.ExecutionFile)
		exeFile, err := os.Create(exeFileName)

		if err != nil {
			log.Printf("Function failed: %s", err)
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		defer exeFile.Close()

		// Write the function into the execution file
		if _, err = exeFile.WriteString(code); err != nil {
			log.Printf("Function failed: %s", err)
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}

		// Build funtion
		if err = d.BuildFunction(defaultDockerRegistry, userName, functionName, runtime); err != nil {
			log.Printf("Build function failed: %s", err)
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}

		// Register function to configured docker registry
		if err = docker.RegisterFunction(defaultDockerRegistry, userName, functionName); err != nil {
			log.Printf("Register function failed: %s", err)
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}

		// If all the above operation succeeded, the function is created
		// successfully.
		fmt.Fprintf(response, html.FunctionCreatedPage)
	}
}

func IndexPageHandler(response http.ResponseWriter, request *http.Request) {
	userName := getUserName(request)
	if userName != "" {
		//Already logged in, show internal page
		fmt.Fprintf(response, html.InternalPage, userName)
	} else {
		fmt.Fprintf(response, html.IndexPage)
	}
}

func LoginHandler(response http.ResponseWriter, request *http.Request) {
	name := request.FormValue("name")
	pass := request.FormValue("password")
	redirectTarget := "/"
	if name != "" && pass != "" {
		// ... check credentials
		ok, err := checkCredentials(name, pass)
		if !ok {
			http.Error(response, err.Error(), http.StatusInternalServerError)
			return
		}
		setSession(name, response)
		redirectTarget = "/internal"
	}
	http.Redirect(response, request, redirectTarget, http.StatusFound)
}

func InternalPageHandler(response http.ResponseWriter, request *http.Request) {
	userName := getUserName(request)
	if userName != "" {
		fmt.Fprintf(response, html.InternalPage, userName)
	} else {
		http.Redirect(response, request, "/", http.StatusFound)
	}
}

func LogoutHandler(response http.ResponseWriter, request *http.Request) {
	clearSession(response)
	log.Println("Logged out")
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

func checkCredentials(name string, pass string) (bool, error) {
	var l *ldap.Conn
	var err error

	servers := []string{"ds.symcpe.net"}
	port := 636
	retries := 3
	username := fmt.Sprintf("uid=%s,ou=People,dc=mgmt,dc=symcpe,dc=net", name)

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
