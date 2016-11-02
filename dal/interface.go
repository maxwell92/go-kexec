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

	// If the function does not exist, insert one,
	// otherwise, update it.
	//
	// Returns: (int64) insert row id,
	//          (int64) # of rows influenced,
	//          (error) if there is one
	PutFunction(userName, funcName, funcContent string, userId int64) (int64, int64, error)
	GetFunction(userName, functionName string) (string, error)
}
