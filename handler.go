package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/wayn3h0/go-uuid"
	"github.com/xuant/go-kexec/dal"
	"github.com/xuant/go-kexec/docker"
	"github.com/xuant/go-kexec/html"
	"gopkg.in/ldap.v2"
)

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

		// Put authenticated user into DB
		insertId, rowCnt, err := putUserIfNotExistedInDB(a, "", name)
		if err != nil {
			http.Redirect(response, request, redirectTarget, http.StatusFound)
			return nil
		}

		if rowCnt > 0 {
			log.Printf("Successfully put user into DB, uid = %d", insertId)
		} else {
			log.Printf("User %s already in DB.", name)
		}

		setSession(a, name, response)
		redirectTarget = "/internal"
	}
	http.Redirect(response, request, redirectTarget, http.StatusFound)
	return nil
}

func LogoutHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	clearSession(response)
	log.Println("Logged out")
	http.Redirect(response, request, "/", http.StatusFound)
	return nil
}

func InternalPageHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	namespace := "default"
	userName := getUserName(a, request)
	if userName != "" {
		functions, err := getUserFunctions(a, namespace, userName, -1)
		if err != nil {
			fmt.Fprintf(response, html.InternalPage, userName, err)
			return nil
		}

		// Functions to be listed. (Only 3 of them if there are more than 3 functions)
		funcToBeListed := make([]string, 3)
		for i := 0; i < 3; i++ {
			if i < len(functions) {
				funcToBeListed[i] = functions[i].Name
			}
		}

		fmt.Fprintf(response, html.InternalPage, userName, funcToBeListed[0], funcToBeListed[1], funcToBeListed[2])
	} else {
		http.Redirect(response, request, "/", http.StatusFound)
	}
	return nil
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

		// Create a time based uuid as part of the context directory name
		uuid, err := uuid.NewTimeBased()

		if err != nil {
			log.Println("Failed to create uuid for function call.")
			return StatusError{http.StatusInternalServerError, err}
		}

		uuidStr := uuid.String()
		userCtx := userName + "-" + uuidStr

		// Create the execution file for the function
		ctxDir := filepath.Join(docker.IBContext, userCtx)

		if err := os.Mkdir(ctxDir, os.ModePerm); err != nil {
			return StatusError{http.StatusInternalServerError, err}
		}

		exeFileName := filepath.Join(ctxDir, docker.ExecutionFile)
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
		if err = a.d.BuildFunction(a.conf.DockerRegistry, userName, functionName, runtime, ctxDir); err != nil {
			log.Println("Build function failed")
			return StatusError{http.StatusInternalServerError, err}
		}

		// Register function to configured docker registry
		if err = docker.RegisterFunction(a.conf.DockerRegistry, userName, functionName); err != nil {
			log.Println("Register function failed")
			return StatusError{http.StatusInternalServerError, err}
		}

		// Put function into db
		if err = putUserFunction(a, userName, functionName, code, -1); err != nil {
			log.Println("Failed to put function into DB")
			return StatusError{http.StatusInternalServerError, err}
		}

		// If all the above operation succeeded, the function is created
		// successfully.
		fmt.Fprintf(response, html.FunctionCreatedPage)
	}
	return nil
}

func CallHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	functionName := getFunctionName(request)

	if userName == "" || functionName == "" {

		// Empty username is not allowed to call function
		http.Redirect(response, request, "/", http.StatusFound)

	} else {
		if _, _, err := callFunction(a, userName, functionName); err != nil {
			return StatusError{http.StatusInternalServerError, err}
		}

		fmt.Fprintf(response, html.FunctionCalledPage)
	}
	return nil
}

func CallFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	vars := mux.Vars(request)
	userName := vars["username"]
	functionName := vars["function"]

	uuidStr, nsName, err := callFunction(a, userName, functionName)
	if err != nil {
		return StatusError{http.StatusInternalServerError, err}
	}
	// Wait for job to complete
	// TODO: check for job completion instead of wait 30s.
	time.Sleep(30 * time.Second)

	funcLog, err := a.k.GetFunctionLog(functionName, uuidStr, nsName)
	if err != nil {
		return StatusError{http.StatusInternalServerError, err}
	}
	log.Printf("Function Log:\n %s", string(funcLog))
	return nil
}

func callFunction(a *appContext, userName, functionName string) (string, string, error) {
	// create a uuid for each function call. This uuid can be
	// seen as the execution id for the function (notice there
	// are multiple executions for a single function)
	uuid, err := uuid.NewTimeBased()

	if err != nil {
		log.Println("Failed to create uuid for function call.")
		return "", "", err
	}

	uuidStr := uuid.String() // uuidStr needed when fetching log

	// Create a namespace for the user and run the job
	// in that namespace
	nsName := strings.Replace(userName, "_", "-", -1) + "-serverless"
	if _, err := a.k.CreateUserNamespaceIfNotExist(nsName); err != nil {
		log.Println("Failed to get/create user namespace", nsName)
		return "", "", err
	}
	jobname := functionName + "-" + uuidStr
	image := a.conf.DockerRegistry + "/" + userName + "/" + functionName
	labels := make(map[string]string)

	if err = a.k.CallFunction(jobname, image, nsName, labels); err != nil {
		log.Println("Failed to call function", functionName)
		return "", "", err
	}
	return uuidStr, nsName, nil
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

func getFunctionName(request *http.Request) string {
	return "Not implemented yet."
}

func putUserIfNotExistedInDB(a *appContext, groupName, userName string) (int64, int64, error) {
	return a.dal.PutUserIfNotExisted(groupName, userName)
}

func getUserFunctions(a *appContext, namespace, username string, userId int64) ([]*dal.Function, error) {
	return a.dal.ListFunctionsOfUser(namespace, username, userId)
}

func putUserFunction(a *appContext, username, funcName, funcContent string, userId int64) error {
	_, _, err := a.dal.PutFunctionIfNotExisted(username, funcName, funcContent, -1)
	return err
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
