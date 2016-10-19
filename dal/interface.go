package dal

type DAL interface {
	// List functions created by a user
	ListFunctionsOfUser(namespace, username string, userId int64) ([]*Function, error)

	// Insert user into DB if not existed.
	//
	// Returns: (int64) insert row id,
	//          (int64) # of rows influenced,
	//          (error) if there is one
	PutUserIfNotExisted(groupName, userName string) (int64, int64, error)

	// Insert function into DB if not existed.
	//
	// Returns: (int64) insert row id,
	//          (int64) # of rows influenced,
	//          (error) if there is one
	PutFunctionIfNotExisted(userName, funcName, funcContent string, userId int64) (int64, int64, error)
}
