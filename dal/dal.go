package dal

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

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
	return fmt.Sprintf("%s:%s@tcp(%s:3306)/?parseTime=true", c.username, c.password, c.dbhost)
}

type MySQL struct {
	*sql.DB

	usersTable      string
	functionsTable  string
	executionsTable string
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

	// Create the users table if not already existed
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

	// Create a unique index on (name) column of users table
	_, err = db.Exec(fmt.Sprintf("ALTER TABLE %s ADD UNIQUE (name)", config.usersTable))

	if err != nil {
		return nil, err
	}

	// Create the functions table if not already existed
	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s ( 
		f_id INT NOT NULL AUTO_INCREMENT, 
		u_id INT NOT NULL,
		name VARCHAR(255) NOT NULL,
		content TEXT, 
		created TIMESTAMP, 
		updated TIMESTAMP, 
		PRIMARY KEY (f_id), 
		FOREIGN KEY (u_id) REFERENCES %s(u_id)
	)`, config.functionsTable, config.usersTable))

	if err != nil {
		return nil, err
	}

	// Create the executions table if not already existed
	_, err = db.Exec(fmt.Sprintf(`
	CREATE TABLE IF NOT EXISTS %s (
		e_id INT NOT NULL AUTO_INCREMENT, 
		f_id INT NOT NULL,
		uuid VARCHAR(255) NOT NULL,
		log TEXT, 
		created TIMESTAMP, 
		PRIMARY KEY (e_id), 
		FOREIGN KEY (f_id) REFERENCES %s(f_id)
	)`, config.executionsTable, config.functionsTable))

	if err != nil {
		return nil, err
	}

	return &MySQL{
		db,
		config.usersTable,
		config.functionsTable,
		config.executionsTable,
	}, nil
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
func (dal *MySQL) ListFunctionsOfUser(namespace, username string, userId int64) ([]*Function, error) {

	uid := userId

	if uid < 0 && username == "" {
		return nil, errors.New("Either userName or userId should be valid")
	}

	if uid < 0 {
		err := dal.QueryRow(fmt.Sprintf("SELECT u_id FROM %s WHERE name = ?", dal.usersTable), username).Scan(&uid)
		if err != nil {
			return nil, err
		}
	}

	funcList := make([]*Function, 0, 5)

	stmt, err := dal.Prepare(fmt.Sprintf(
		"SELECT f_id, name, content, created FROM %s WHERE u_id = ?",
		dal.functionsTable))
	if err != nil {
		fmt.Println(err)
		return funcList, err
	}
	defer stmt.Close()

	fmt.Printf("stmt: %s\n", stmt)
	rows, err := stmt.Query(uid)
	if err != nil {
		return funcList, err
	}
	defer rows.Close()

	for rows.Next() {
		function := &Function{
			ID:      -1,
			UserID:  uid,
			Name:    "",
			Content: "",
			Created: time.Time{},
			Updated: time.Time{},
		}

		err := rows.Scan(&function.ID, &function.Name, &function.Content, &function.Created)
		if err != nil {
			return funcList, err
		}

		funcList = append(funcList, function)
	}

	if err = rows.Err(); err != nil {
		return funcList, err
	}

	return funcList, nil
}

// Put group
func (dal *MySQL) PutGroup(groupName string) error {
	return errors.New("Not implemented yet.")
}

// PutUserIfNotExists inserts user into DB if the user
// is not already inserted. The caller is responsible for
// making sure `userName` is not empty.
func (dal *MySQL) PutUserIfNotExisted(groupName, userName string) (int64, int64, error) {
	stmt, err := dal.Prepare(fmt.Sprintf(
		"INSERT IGNORE INTO %s (name, created) VALUES (?, ?)",
		dal.usersTable))

	if err != nil {
		return -1, -1, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(userName, time.Now().Format(time.RFC3339))
	if err != nil {
		return -1, -1, err
	}

	lastId, err := res.LastInsertId()
	if err != nil {
		return -1, -1, err
	}

	rowCnt, err := res.RowsAffected()
	if err != nil {
		return -1, -1, err
	}

	return lastId, rowCnt, nil
}

// is not already inserted.
//
// When both `userName` and `userId` are not empty, the function check
// userId first.
func (dal *MySQL) PutFunctionIfNotExisted(userName, funcName, funcContent string, userId int64) (int64, int64, error) {

	uid := userId

	if uid < 0 && userName == "" {
		return -1, -1, errors.New("Either userName or userId should be valid")
	}

	if uid < 0 {
		err := dal.QueryRow(fmt.Sprintf("SELECT u_id FROM %s WHERE name = ?", dal.usersTable), userName).Scan(&uid)
		if err != nil {
			return -1, -1, err
		}
	}

	stmt, err := dal.Prepare(fmt.Sprintf(
		"INSERT INTO %s (u_id, name, content, created) VALUES (?, ?, ?, ?)",
		dal.functionsTable))

	if err != nil {
		return -1, -1, err
	}

	res, err := stmt.Exec(uid, funcName, funcContent, time.Now().Format(time.RFC3339))
	if err != nil {
		return -1, -1, err
	}

	lastId, err := res.LastInsertId()
	if err != nil {
		return -1, -1, err
	}

	rowCnt, err := res.RowsAffected()
	if err != nil {
		return -1, -1, err
	}

	return lastId, rowCnt, nil
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

// Careful with this function, it drops your entire database.
// Only used for test purpose.
func (dal *MySQL) ClearDatabase() error {
	if _, err := dal.Exec(fmt.Sprintf("DELETE FROM %s", dal.executionsTable)); err != nil {
		return err
	}

	if _, err := dal.Exec(fmt.Sprintf("DELETE FROM %s", dal.functionsTable)); err != nil {
		return err
	}

	if _, err := dal.Exec(fmt.Sprintf("DELETE FROM %s", dal.usersTable)); err != nil {
		return err
	}

	return nil
}
