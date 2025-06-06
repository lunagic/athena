package athenatools_test

import (
	"testing"

	"github.com/lunagic/athena/athenatools"
	"gotest.tools/v3/assert"
)

func TestFilter(t *testing.T) {
	users := []User{
		{
			Name: "Aaron",
			Age:  33,
		},
		{
			Name: "Andy",
			Age:  10,
		},
	}

	assert.DeepEqual(
		t,
		[]User{
			UserAaron,
		},
		athenatools.Filter(
			users,
			func(user User) bool {
				return user.Age >= 18
			},
		),
	)
}
