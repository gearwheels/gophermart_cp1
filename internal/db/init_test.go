package db

import (
	"testing"

	"github.com/gearwheels/gophermart_cp1/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func TestSQLite_AutoMigrateSmoke(t *testing.T) {
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	require.NoError(t, gdb.AutoMigrate(&model.User{}, &model.Order{}, &model.Withdrawal{}))

	// sanity: can insert into migrated tables
	u := &model.User{Login: "u", PasswordHash: "x"}
	require.NoError(t, gdb.Create(u).Error)
}

