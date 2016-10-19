package main

import (
	"github.com/gorilla/securecookie"
	"github.com/xuant/go-kexec/docker"
	"github.com/xuant/go-kexec/kexec"
	"net/http"
)

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
