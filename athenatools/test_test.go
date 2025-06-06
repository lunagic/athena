package athenatools_test

type User struct {
	Name string
	Age  int
}

var UserAaron = User{
	Name: "Aaron",
	Age:  33,
}

var UserAndy = User{
	Name: "Andy",
	Age:  10,
}

var users = []User{
	UserAaron,
	UserAndy,
}
