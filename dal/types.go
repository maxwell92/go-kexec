package dal

import "time"

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
	Name          string
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
