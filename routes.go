package main

import (
	"log"
	"net/http"

	"github.com/gorilla/mux"
)

type Route struct {
	Name    string
	Method  string
	Pattern string
	Handler appRouteHandler
}

type Routes []Route

func (ah appHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	err := ah.H(ah.appContext, w, r)
	if err != nil {
		switch e := err.(type) {
		case Error:
			// We can retrieve the status here and write out a specific
			// HTTP status code.
			log.Printf("HTTP %d - %s", e.Status(), e)
			http.Error(w, e.Message(), e.Status())
		default:
			// Any error types we don't specifically look out for default
			// to serving a HTTP 500
			http.Error(w, http.StatusText(http.StatusInternalServerError),
				http.StatusInternalServerError)
		}
	}
}

func NewRouter(context *appContext) *mux.Router {

	router := mux.NewRouter()
	for _, route := range routes {
		router.
			Methods(route.Method).
			Path(route.Pattern).
			Name(route.Name).
			Handler(appHandler{context, route.Handler})
	}

	router.PathPrefix("/").Handler(http.FileServer(http.Dir(context.conf.FileServerDir)))
	return router
}

var routes = Routes{
	Route{
		"Index",
		"GET",
		"/",
		IndexPageHandler,
	},
	Route{
		"Login",
		"POST",
		"/login",
		LoginHandler,
	},
	Route{
		"Logout",
		"GET",
		"/logout",
		LogoutHandler,
	},
	Route{
		"Internal",
		"GET",
		"/internal",
		InternalPageHandler,
	},
	Route{
		"Create",
		"POST",
		"/create",
		CreateFunctionHandler,
	},
	Route{
		"Call",
		"GET",
		"/call",
		CallHandler,
	},
	Route{
		"Call",
		"POST",
		"/call/{username}/{function}",
		CallFunctionHandler,
	},
}
