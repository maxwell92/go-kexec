package dal

import (
	"log"
	"testing"
)

func TestMain(m *testing.M) {

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

	testUsername := "TestUser"
	testFuncname := "TestFunc"
	testFuncContent := `
	def foo():
		print("Hello world.")
	foo()
	`

	log.Printf("Inserting user...")
	lastId, rowCount, err := dal.PutUserIfNotExisted("", testUsername)
	if err != nil {
		panic(err)
	}

	log.Printf("Last ID: %d, Rows affected: %d", lastId, rowCount)

	log.Printf("Inserting function...")
	lastId, rowCount, err = dal.PutFunctionIfNotExisted(testUsername, testFuncname, testFuncContent, -1)
	if err != nil {
		panic(err)
	}

	log.Printf("Last ID: %d, Rows affected: %d", lastId, rowCount)
}
