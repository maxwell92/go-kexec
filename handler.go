package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/gorilla/mux"
	"github.com/wayn3h0/go-uuid"
	"github.com/xuant/go-kexec/docker"
	"gopkg.in/ldap.v2"
)

var (
	MessageCreateFunctionFailed = "Failed to create function"

	MessageCallFunctionFailed = "Failed to call function"

	MessageInternalServerError = "Server Error"
)

func IndexPageHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName != "" {
		//Already logged in, show dashboard
		//TODO: redirect or call the handler directly
		return DashboardHandler(a, response, request)
	} else {
		t := template.Must(template.ParseFiles("html/login.html"))
		t.Execute(response, nil)
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
			t := template.Must(template.ParseFiles("html/login.html"))
			t.Execute(response, &LoginPage{LoginErr: true, ErrMsg: errMsg})
			return nil
		}

		// Put authenticated user into DB
		insertId, rowCnt, err := putUserIfNotExistedInDB(a, "", name)

		// Return internal server error if DB operation failed
		if err != nil {
			return StatusError{http.StatusInternalServerError, err, MessageInternalServerError}
		}

		if rowCnt > 0 {
			log.Printf("Successfully put user into DB, uid = %d", insertId)
		} else {
			log.Printf("User %s already in DB.", name)
		}

		setSession(a, name, response)
		redirectTarget = "/dashboard"
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

func DashboardHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	namespace := "default"
	userName := getUserName(a, request)
	t := template.Must(template.ParseFiles("html/dashboard.html"))
	if userName != "" {
		functions, err := getUserFunctions(a, namespace, userName, -1)
		// Cannot list the function, return a page with no function name
		if err != nil {
			log.Println("Cannot list functions for", userName)
			t.Execute(response, &DashboardPage{Username: userName})
			return nil
		}

		t.Execute(response, &DashboardPage{Username: userName, Functions: functions})
	} else {
		http.Redirect(response, request, "/", http.StatusFound)
	}
	return nil
}

func CreateFuncPageHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName == "" {
		http.Redirect(response, request, "/", http.StatusFound)
	} else {
		t := template.Must(template.ParseFiles("html/configure_func.html"))
		t.Execute(response, nil)
	}
	return nil
}

func EditFuncPageHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	if userName == "" {
		http.Redirect(response, request, "/", http.StatusFound)
	} else {
		vars := mux.Vars(request)
		functionName := vars["function"]

		content, err := a.dal.GetFunction(userName, functionName)
		if err != nil {
			log.Println("Cannot get function", functionName)
			return StatusError{http.StatusInternalServerError, err, MessageInternalServerError}
		}
		t := template.Must(template.ParseFiles("html/configure_func.html"))
		t.Execute(response, &ConfigFuncPage{
			EnableFuncName: false,
			FuncName:       functionName,
			FuncRuntime:    "python27",
			FuncContent:    content})
	}
	return nil
}

func DeleteFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
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
			return StatusError{http.StatusFound, err, MessageCreateFunctionFailed}
		}

		newCode := formatCode(code, functionName)
		log.Printf("Code uploaded:\n%s", newCode)
		log.Printf("Start creating function \"%s\" with runtime \"%s\"", functionName, runtime)

		// Create a time based uuid as part of the context directory name
		uuid, err := uuid.NewTimeBased()

		if err != nil {
			log.Println("Failed to create uuid for function call.")
			return StatusError{http.StatusFound, err, MessageCreateFunctionFailed}
		}

		uuidStr := uuid.String()
		userCtx := userName + "-" + uuidStr

		// Create the execution file for the function
		ctxDir := filepath.Join(docker.IBContext, userCtx)

		if err := os.Mkdir(ctxDir, os.ModePerm); err != nil {
			return StatusError{http.StatusFound, err, MessageCreateFunctionFailed}
		}

		exeFileName := filepath.Join(ctxDir, docker.ExecutionFile)
		exeFile, err := os.Create(exeFileName)

		if err != nil {
			return StatusError{http.StatusFound, err, MessageCreateFunctionFailed}
		}
		defer exeFile.Close()

		// Write the function into the execution file
		if _, err = exeFile.WriteString(newCode); err != nil {
			return StatusError{http.StatusFound, err, MessageCreateFunctionFailed}
		}

		// Build funtion
		if err = a.d.BuildFunction(a.conf.DockerRegistry, userName, functionName, runtime, ctxDir); err != nil {
			log.Println("Build function failed")
			return StatusError{http.StatusFound, err, MessageCreateFunctionFailed}
		}

		// Register function to configured docker registry
		if err = docker.RegisterFunction(a.conf.DockerRegistry, userName, functionName); err != nil {
			log.Println("Register function failed")
			return StatusError{http.StatusFound, err, MessageCreateFunctionFailed}
		}

		// Put function into db
		if err = putUserFunction(a, userName, functionName, code, -1); err != nil {
			log.Println("Failed to put function into DB")
			return StatusError{http.StatusFound, err, MessageCreateFunctionFailed}
		}

		// If all the above operation succeeded, the function is created
		// successfully.
		t := template.Must(template.ParseFiles("html/func_created.html"))
		t.Execute(response, nil)
	}
	return nil
}

func CallHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	userName := getUserName(a, request)
	vars := mux.Vars(request)
	functionName := vars["function"]
	params := request.FormValue("params")

	if userName == "" {
		// Empty username is not allowed to call function
		http.Redirect(response, request, "/", http.StatusFound)
	} else {
		if functionName == "" {
			return StatusError{http.StatusFound, errors.New("Empty function name"), MessageCallFunctionFailed}
		}
		if params == "" {
			log.Println("Calling function", functionName)
		} else {
			log.Println("Calling function", functionName, "with parameters", params)
		}
		funcLog, err := callFunction(a, userName, functionName, params)
		if err != nil {
			return StatusError{http.StatusFound, err, MessageCallFunctionFailed}
		}

		t := template.Must(template.ParseFiles("html/func_called.html"))
		t.Execute(response, &CallResult{Log: funcLog})
	}
	return nil
}

func CallFunctionHandler(a *appContext, response http.ResponseWriter, request *http.Request) error {
	vars := mux.Vars(request)
	userName := vars["username"]
	functionName := vars["function"]

	// Get function parameters from request body
	params, err := ioutil.ReadAll(request.Body)
	if err != nil {
		return StatusError{http.StatusFound, err, MessageCallFunctionFailed}
	}
	paramsStr := string(params)
	if paramsStr == "" {
		log.Println("Calling function", functionName)
	} else {
		log.Println("Calling function", functionName, "with parameters", paramsStr)
	}

	// Call function. This will create a job in OpenShift
	funcLog, err := callFunction(a, userName, functionName, paramsStr)
	if err != nil {
		return StatusError{http.StatusFound, err, MessageCallFunctionFailed}
	}
	// Write to response
	fmt.Fprintf(response, string(funcLog))
	return nil
}

func callFunction(a *appContext, userName, functionName, params string) (string, error) {
	// create a uuid for each function call. This uuid can be
	// seen as the execution id for the function (notice there
	// are multiple executions for a single function)
	uuid, err := uuid.NewTimeBased()

	if err != nil {
		log.Println("Failed to create uuid for function call.")
		return "", err
	}

	uuidStr := uuid.String() // uuidStr needed when fetching log

	// Create a namespace for the user and run the job
	// in that namespace
	nsName := strings.Replace(userName, "_", "-", -1) + "-serverless"
	if _, err := a.k.CreateUserNamespaceIfNotExist(nsName); err != nil {
		log.Println("Failed to get/create user namespace", nsName)
		return "", err
	}
	jobName := functionName + "-" + uuidStr
	image := a.conf.DockerRegistry + "/" + userName + "/" + functionName
	labels := make(map[string]string)

	if err := a.k.CreateFunctionJob(jobName, image, params, nsName, labels); err != nil {
		log.Println("Failed to call function", functionName)
		return "", err
	}
	// Wait for job to complete
	if err := a.k.WaitForPodComplete(jobName, nsName); err != nil {
		return "", err
	}

	funcLog, err := a.k.GetFunctionLog(jobName, nsName)
	if err != nil {
		return "", err
	}
	log.Printf("Function Log:\n %s", string(funcLog))
	return string(funcLog), nil
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

func getUserFunctions(a *appContext, namespace, username string, userId int64) ([]*FunctionRow, error) {
	functions, err := a.dal.ListFunctionsOfUser(namespace, username, userId)
	if err != nil {
		return nil, err
	}
	l := len(functions)
	funcToBeListed := make([]*FunctionRow, 0, l)
	for i := 0; i < l; i++ {
		f := &FunctionRow{
			FuncName:    functions[i].Name,
			Owner:       username,
			UpdatedTime: functions[i].Updated,
		}
		funcToBeListed = append(funcToBeListed, f)
	}
	return funcToBeListed, nil
}

func putUserFunction(a *appContext, username, funcName, funcContent string, userId int64) error {
	_, _, err := a.dal.PutFunction(username, funcName, funcContent, -1)
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

// Add imports and the remaining code
func formatCode(code, functionName string) string {
	return fmt.Sprintf("import json\nimport os\n\n"+
		"%s\n\n"+
		"params = os.environ[\"SERVERLESS_PARAMS\"]\n"+
		"%s(json.loads(params))\n", code, functionName)
}
