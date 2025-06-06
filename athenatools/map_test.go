package athenatools_test

import (
	"testing"

	"github.com/lunagic/athena/athenatools"
	"gotest.tools/v3/assert"
)

func TestMap(t *testing.T) {
	assert.DeepEqual(
		t,
		[]bool{
			true,
			false,
		},
		athenatools.Map(
			users,
			func(user User) bool {
				return user.Age >= 18
			},
		),
	)
}
