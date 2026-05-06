package accrual

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestClient_GetOrder_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, "/api/orders/123", r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"order":"123","status":"PROCESSED","accrual":500}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	oi, err := c.GetOrder(context.Background(), "123")
	require.NoError(t, err)
	require.Equal(t, "123", oi.Order)
	require.Equal(t, StatusProcessed, oi.Status)
	require.NotNil(t, oi.Accrual)
	require.Equal(t, 500.0, *oi.Accrual)
}

func TestClient_GetOrder_204(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetOrder(context.Background(), "123")
	require.ErrorIs(t, err, ErrNotRegistered)
}

func TestClient_GetOrder_429_RetryAfter(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "2")
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetOrder(context.Background(), "123")
	var tme *TooManyError
	require.ErrorAs(t, err, &tme)
	require.Equal(t, 2*time.Second, tme.RetryAfter)
}

func TestClient_GetOrder_UnexpectedStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL)
	_, err := c.GetOrder(context.Background(), "123")
	require.Error(t, err)
}

func TestClient_Disabled(t *testing.T) {
	c := NewClient("")
	require.False(t, c.Enabled())
	_, err := c.GetOrder(context.Background(), "123")
	require.Error(t, err)
}

