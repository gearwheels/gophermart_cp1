package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	settings "github.com/gearwheels/gophermart_cp1/internal/config"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
)

func TestBuildCookieValue_AndMiddleware_AllowsValidCookie(t *testing.T) {
	settings.AppConfig = &settings.Config{SecretKeyForJWT: "test-secret"}

	e := echo.New()
	e.Use(Middleware(nil))

	e.GET("/api/user/balance", func(c echo.Context) error {
		id, ok := GetUserID(c)
		require.True(t, ok)
		require.Equal(t, int64(123), id)
		return c.NoContent(http.StatusOK)
	})

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	req.AddCookie(&http.Cookie{Name: "session", Value: BuildCookieValue(123)})
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
}

func TestMiddleware_RejectsMissingCookie(t *testing.T) {
	settings.AppConfig = &settings.Config{SecretKeyForJWT: "test-secret"}

	e := echo.New()
	e.Use(Middleware(nil))
	e.GET("/api/user/balance", func(c echo.Context) error { return c.NoContent(http.StatusOK) })

	req := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	rec := httptest.NewRecorder()

	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

