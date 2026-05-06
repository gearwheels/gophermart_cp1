package accrual_worker

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gearwheels/gophermart_cp1/internal/accrual"
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

func TestWorker_AppliesAccrualOnce(t *testing.T) {
	// заглушка системы начисления
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"79927398713","status":"PROCESSED","accrual":10}`))
	}))
	defer srv.Close()

	db := newTestDB(t)
	u := &model.User{Login: "u", PasswordHash: "x"}
	require.NoError(t, db.Create(u).Error)

	o := &model.Order{Number: "79927398713", UserID: u.ID, Status: model.OrderStatusNew, UploadedAt: time.Now().UTC()}
	require.NoError(t, db.Create(o).Error)

	w := New(db, accrual.NewClient(srv.URL), 10)

	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	go w.Run(ctx)

	<-ctx.Done()

	var u2 model.User
	require.NoError(t, db.First(&u2, u.ID).Error)
	require.Equal(t, 10.0, u2.CurrentBalance)

	var o2 model.Order
	require.NoError(t, db.First(&o2, "number = ?", o.Number).Error)
	require.Equal(t, model.OrderStatusProcessed, o2.Status)
	require.True(t, o2.AccrualApplied)

	// убеждаемся, что повторный запуск не начислит баллы дважды
	ctx2, cancel2 := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel2()
	go w.Run(ctx2)
	<-ctx2.Done()

	var u3 model.User
	require.NoError(t, db.First(&u3, u.ID).Error)
	require.Equal(t, 10.0, u3.CurrentBalance)
}

func TestWorker_AccrualNotRegistered_LeavesOrderNew(t *testing.T) {
	// HTTP 204 от системы начисления означает «заказ не зарегистрирован»
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	db := newTestDB(t)
	u := &model.User{Login: "u", PasswordHash: "x"}
	require.NoError(t, db.Create(u).Error)
	o := &model.Order{Number: "123", UserID: u.ID, Status: model.OrderStatusNew, UploadedAt: time.Now().UTC()}
	require.NoError(t, db.Create(o).Error)

	w := New(db, accrual.NewClient(srv.URL), 10)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	go w.Run(ctx)
	<-ctx.Done()

	var o2 model.Order
	require.NoError(t, db.First(&o2, "number = ?", o.Number).Error)
	require.Equal(t, model.OrderStatusNew, o2.Status)

	var u2 model.User
	require.NoError(t, db.First(&u2, u.ID).Error)
	require.Equal(t, 0.0, u2.CurrentBalance)
}

func TestWorker_RegisteredStatus_KeepsOrderNew(t *testing.T) {
	// Accrual system returns REGISTERED — order is known but not yet being calculated.
	// The gophermart order must remain NEW (not transition to PROCESSING).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"79927398713","status":"REGISTERED"}`))
	}))
	defer srv.Close()

	db := newTestDB(t)
	u := &model.User{Login: "u", PasswordHash: "x"}
	require.NoError(t, db.Create(u).Error)
	o := &model.Order{Number: "79927398713", UserID: u.ID, Status: model.OrderStatusNew, UploadedAt: time.Now().UTC()}
	require.NoError(t, db.Create(o).Error)

	w := New(db, accrual.NewClient(srv.URL), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	go w.Run(ctx)
	<-ctx.Done()

	var o2 model.Order
	require.NoError(t, db.First(&o2, "number = ?", o.Number).Error)
	require.Equal(t, model.OrderStatusNew, o2.Status, "статус REGISTERED от системы начисления должен оставлять заказ в NEW")

	var u2 model.User
	require.NoError(t, db.First(&u2, u.ID).Error)
	require.Equal(t, 0.0, u2.CurrentBalance, "при статусе REGISTERED баллы начисляться не должны")
}

func TestWorker_InvalidOrder_MarksInvalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"999","status":"INVALID"}`))
	}))
	defer srv.Close()

	db := newTestDB(t)
	u := &model.User{Login: "u", PasswordHash: "x"}
	require.NoError(t, db.Create(u).Error)
	o := &model.Order{Number: "999", UserID: u.ID, Status: model.OrderStatusNew, UploadedAt: time.Now().UTC()}
	require.NoError(t, db.Create(o).Error)

	w := New(db, accrual.NewClient(srv.URL), 10)
	ctx, cancel := context.WithTimeout(context.Background(), 250*time.Millisecond)
	defer cancel()
	go w.Run(ctx)
	<-ctx.Done()

	var o2 model.Order
	require.NoError(t, db.First(&o2, "number = ?", o.Number).Error)
	require.Equal(t, model.OrderStatusInvalid, o2.Status)
	require.False(t, o2.AccrualApplied)
}

