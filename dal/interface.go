package dal

type DAL interface {
	// List all groups
	ListGroups(groupName string) ([]Group, error)

	// List all users inside a group
	ListUsersOfGroup(groupName string) ([]User, error)

	// List all functions created by a user
	ListFunctionsOfUser(namespace, username string) ([]Function, error)

	// Put group
	PutGroup(groupName string) error

	// Put user
	PutUser(groupName, userName string) error

	// Put function
	PutFunction(userName, funcName string) error

	// Get function content
	GetFunctionContent(userName, funcName string) (string, error)
}
