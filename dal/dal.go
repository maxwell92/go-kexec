package main

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type Group struct {
	ID      string
	Name    string
	Created time.Time
	Users   []User
}

type User struct {
	ID        string
	Name      string
	Created   time.Time
	Functions []Function
}

type Function struct {
	ID               string
	User             string
	Content          string
	Log              string
	ExecutionPod     string
	LastExecutionPod string
	Created          time.Time
	Updated          time.Time
	LastExecution    time.Time
}

type DalConfig struct {
	dbhost   string
	dbname   string
	username string
	password string
}

func (c *DalConfig) getDB() string {
	return fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", c.username, c.password, c.dbhost, c.dbname)
}

type MySQL struct {
	*sql.DB
}

func main() {
	config := &DalConfig{
		dbhost:   "100.73.145.91",
		dbname:   "kexec",
		username: "kexec",
		password: "password",
	}

	dal, err := NewMySQL(config)

	if err != nil {
		panic(err)
	}

	if err = dal.Ping(); err != nil {
		panic(err)
	}
}

func NewMySQL(config *DalConfig) (*MySQL, error) {
	fmt.Println(config.getDB())
	db, err := sql.Open("mysql", config.getDB())
	if err != nil {
		return nil, err
	}

	return &MySQL{db}, nil
}

func (dal *MySQL) ListGroups(groupName string) ([]Group, error) {
	return nil, errors.New("Not implemented yet.")
}

// List all users inside a group
func (dal *MySQL) ListUsersOfGroup(groupName string) ([]User, error) {
	return nil, errors.New("Not implemented yet.")
}

// List all functions created by a user
func (dal *MySQL) ListFunctionsOfUser(namespace, username string) ([]Function, error) {
	return nil, errors.New("Not implemented yet.")
}

// Put group
func (dal *MySQL) PutGroup(groupName string) error {
	return errors.New("Not implemented yet.")
}

// Put user
func (dal *MySQL) PutUser(groupName, userName string) error {
	return errors.New("Not implemented yet.")
}

// Put function
func (dal *MySQL) PutFunction(userName, funcName string) error {
	return errors.New("Not implemented yet.")
}

// Get function content
func (dal *MySQL) GetFunctionContent(userName, funcName string) (string, error) {
	return "", errors.New("Not implemented yet.")
}
