package enum_test

import (
	"testing"

	"github.com/robdavid/img-pin/pkgs/digester/types"
	"github.com/robdavid/img-pin/pkgs/enum"
	"github.com/stretchr/testify/assert"
)

func TestEnumFromString(t *testing.T) {
	assert := assert.New(t)
	n, err := enum.FromString[types.UpdateMethod]("sync")
	assert.NoError(err)
	assert.Equal(types.UpdateMethod(2), n)
}

func TestInvalidEnumFromString(t *testing.T) {
	assert := assert.New(t)
	_, err := enum.FromString[types.UpdateMethod]("nuke")
	assert.ErrorIs(err, enum.ErrNotValid)
}
