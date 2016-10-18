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
	ID      string
	Name    string
	Created time.Time
}

type Function struct {
	ID            string
	UserID        string
	Content       string
	Created       time.Time
	Updated       time.Time
	LastExecution time.Time
}

type FunctionExecution struct {
	ID         string
	FunctionID string
	Log        string
	Timestamp  time.Time
}

type DalConfig struct {
	// data source
	dbhost   string
	username string
	password string

	// db
	dbname string

	// tables
	usersTable      string
	functionsTable  string
	executionsTable string
}

func (c *DalConfig) getDataSourceName() string {
	//	return fmt.Sprintf("%s:%s@tcp(%s:3306)/%s", c.username, c.password, c.dbhost, c.dbname)
	return fmt.Sprintf("%s:%s@tcp(%s:3306)/", c.username, c.password, c.dbhost)
}

type MySQL struct {
	*sql.DB
}

func main() {
	config := &DalConfig{
		dbhost:   "100.73.145.91",
		username: "kexec",
		password: "password",

		dbname: "kexec",

		usersTable:      "users",
		functionsTable:  "functions",
		executionsTable: "executions",
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
	db, err := sql.Open("mysql", config.getDataSourceName())
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s", config.dbname))
	if err != nil {
		return nil, err
	}

	_, err = db.Exec("USE " + config.dbname)
	if err != nil {
		return nil, err
	}

	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s ( 
		u_id INT NOT NULL AUTO_INCREMENT, 
		name VARCHAR(255) NOT NULL, 
		created TIMESTAMP, 
		PRIMARY KEY (u_id)
	)`, config.usersTable))

	if err != nil {
		return nil, err
	}

	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s ( 
		f_id INT NOT NULL AUTO_INCREMENT, 
		u_id INT NOT NULL, 
		content TEXT, 
		created TIMESTAMP, 
		updated TIMESTAMP, 
		PRIMARY KEY (f_id), 
		FOREIGN KEY (u_id) REFERENCES %s(u_id)
	)`, config.functionsTable, config.usersTable))

	if err != nil {
		return nil, err
	}

	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		e_id INT NOT NULL AUTO_INCREMENT, 
		f_id INT NOT NULL, 
		log TEXT, 
		created TIMESTAMP, 
		PRIMARY KEY (e_id), 
		FOREIGN KEY (f_id) REFERENCES %s(f_id)
	)`, config.executionsTable, config.functionsTable))

	if err != nil {
		return nil, err
	}

	return &MySQL{db}, nil
}

// List all groups inside an org
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

// Put user if the user is not yet created
func (dal *MySQL) PutUserIfNotExisted(groupName, userName string) error {
	return errors.New("Not implemented yet.")
}

// Put function if the function is not yet created
func (dal *MySQL) PutFunctionIfNotExisted(userName, funcName string) error {
	return errors.New("Not implemented yet.")
}

// Overwrite function even if it was created already
func (dal *MySQL) PutFunction(userName, funcName string) error {
	return errors.New("Not implemented yet.")
}

func (dal *MySQL) RecordFunctionExecution(userName, funcName, uuid string) error {
	return errors.New("Not implemented yet.")
}

// Get function content
func (dal *MySQL) GetFunctionContent(userName, funcName string) (string, error) {
	return "", errors.New("Not implemented yet.")
}
