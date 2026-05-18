package repopg

import (
	"context"
	"testing"
	"time"

	"github.com/gearwheels/gophermart_cp1/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&model.User{}, &model.Order{}, &model.Withdrawal{}))
	return gdb
}

func TestOrderRepository_GetByUserID_SortsDesc(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	u := &model.User{Login: "u", PasswordHash: "x"}
	require.NoError(t, db.Create(u).Error)

	now := time.Now().UTC()
	o1 := &model.Order{Number: "1", UserID: u.ID, Status: model.OrderStatusNew, UploadedAt: now.Add(-1 * time.Hour)}
	o2 := &model.Order{Number: "2", UserID: u.ID, Status: model.OrderStatusNew, UploadedAt: now}
	require.NoError(t, db.Create(o1).Error)
	require.NoError(t, db.Create(o2).Error)

	r := NewOrderRepository(db)
	got, err := r.GetByUserID(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "2", got[0].Number)
	require.Equal(t, "1", got[1].Number)
}

func TestWithdrawalRepository_GetByUserID_SortsDesc(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	u := &model.User{Login: "u", PasswordHash: "x"}
	require.NoError(t, db.Create(u).Error)

	now := time.Now().UTC()
	w1 := &model.Withdrawal{UserID: u.ID, OrderNumber: "a", Sum: 1, ProcessedAt: now.Add(-1 * time.Hour)}
	w2 := &model.Withdrawal{UserID: u.ID, OrderNumber: "b", Sum: 1, ProcessedAt: now}
	require.NoError(t, db.Create(w1).Error)
	require.NoError(t, db.Create(w2).Error)

	r := NewWithdrawalRepository(db)
	got, err := r.GetByUserID(ctx, u.ID)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "b", got[0].OrderNumber)
	require.Equal(t, "a", got[1].OrderNumber)
}

func TestBaseRepository_CRUD(t *testing.T) {
	db := newTestDB(t)
	ctx := context.Background()

	r := NewRepository[model.User](db)

	u := &model.User{Login: "u", PasswordHash: "x"}
	require.NoError(t, r.Create(ctx, u))
	require.NotZero(t, u.ID)

	got, err := r.GetByID(ctx, uint(u.ID))
	require.NoError(t, err)
	require.Equal(t, "u", got.Login)

	require.NoError(t, r.UpdateFields(ctx, uint(u.ID), map[string]any{"login": "u2"}))
	got2, err := r.FirstWhere(ctx, "id = ?", u.ID)
	require.NoError(t, err)
	require.Equal(t, "u2", got2.Login)

	require.NoError(t, r.Delete(ctx, uint(u.ID)))
	_, err = r.GetByID(ctx, uint(u.ID))
	require.Error(t, err)
}

