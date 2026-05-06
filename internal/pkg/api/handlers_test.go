package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	settings "github.com/gearwheels/gophermart_cp1/internal/config"
	"github.com/gearwheels/gophermart_cp1/internal/db"
	"github.com/gearwheels/gophermart_cp1/internal/model"
	authmid "github.com/gearwheels/gophermart_cp1/internal/middleware/auth"
	"github.com/labstack/echo/v4"
	"github.com/stretchr/testify/require"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&model.User{}, &model.Order{}, &model.Withdrawal{}))
	return gdb
}

func newTestEcho(gdb *gorm.DB) *echo.Echo {
	e := echo.New()
	e.Use(authmid.Middleware(gdb))
	RegisterHandlers(e, NewServer(gdb))
	return e
}

func extractSessionCookieValue(setCookie string) string {
	// "session=value; Path=/; ..."
	parts := strings.Split(setCookie, ";")
	if len(parts) == 0 {
		return ""
	}
	kv := strings.SplitN(strings.TrimSpace(parts[0]), "=", 2)
	if len(kv) != 2 {
		return ""
	}
	return kv[1]
}

func TestHappyPath_Register_Login_Orders_Balance_Withdrawals(t *testing.T) {
	settings.AppConfig = &settings.Config{SecretKeyForJWT: "test-secret"}

	gdb := newTestDB(t)
	_ = db.InitDB // keep package referenced (documents intended db usage)

	e := newTestEcho(gdb)

	// register
	regBody, _ := json.Marshal(Credentials{Login: "u1", Password: "p1"})
	regReq := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	e.ServeHTTP(regRec, regReq)
	require.Equal(t, http.StatusOK, regRec.Code)
	setCookie := regRec.Header().Get("Set-Cookie")
	require.Contains(t, setCookie, "session=")

	sessionVal := extractSessionCookieValue(setCookie)
	require.NotEmpty(t, sessionVal)

	// login
	loginBody, _ := json.Marshal(Credentials{Login: "u1", Password: "p1"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusOK, loginRec.Code)

	// upload order
	orderNumber := "79927398713" // valid Luhn
	upReq := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(orderNumber))
	upReq.Header.Set("Content-Type", "text/plain")
	upReq.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	upRec := httptest.NewRecorder()
	e.ServeHTTP(upRec, upReq)
	require.Equal(t, http.StatusAccepted, upRec.Code)

	// upload same order again => 200
	upReq2 := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(orderNumber))
	upReq2.Header.Set("Content-Type", "text/plain")
	upReq2.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	upRec2 := httptest.NewRecorder()
	e.ServeHTTP(upRec2, upReq2)
	require.Equal(t, http.StatusOK, upRec2.Code)

	// list orders
	listReq := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	listReq.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	listRec := httptest.NewRecorder()
	e.ServeHTTP(listRec, listReq)
	require.Equal(t, http.StatusOK, listRec.Code)

	// balance initially zero
	balReq := httptest.NewRequest(http.MethodGet, "/api/user/balance", nil)
	balReq.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	balRec := httptest.NewRecorder()
	e.ServeHTTP(balRec, balReq)
	require.Equal(t, http.StatusOK, balRec.Code)

	// withdraw insufficient => 402
	withdrawBody, _ := json.Marshal(WithdrawRequest{Order: "2377225624", Sum: 10})
	wdReq := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(withdrawBody))
	wdReq.Header.Set("Content-Type", "application/json")
	wdReq.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	wdRec := httptest.NewRecorder()
	e.ServeHTTP(wdRec, wdReq)
	require.Equal(t, http.StatusPaymentRequired, wdRec.Code)

	// top up user directly and withdraw OK
	require.NoError(t, gdb.Model(&model.User{}).Where("login = ?", "u1").Update("current_balance", 100).Error)

	wdReq2 := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(withdrawBody))
	wdReq2.Header.Set("Content-Type", "application/json")
	wdReq2.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	wdRec2 := httptest.NewRecorder()
	e.ServeHTTP(wdRec2, wdReq2)
	require.Equal(t, http.StatusOK, wdRec2.Code)

	// withdrawals list
	wlReq := httptest.NewRequest(http.MethodGet, "/api/user/withdrawals", nil)
	wlReq.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	wlRec := httptest.NewRecorder()
	e.ServeHTTP(wlRec, wlReq)
	require.Equal(t, http.StatusOK, wlRec.Code)
}

func TestRegister_Conflict(t *testing.T) {
	settings.AppConfig = &settings.Config{SecretKeyForJWT: "test-secret"}
	gdb := newTestDB(t)
	e := newTestEcho(gdb)

	body, _ := json.Marshal(Credentials{Login: "u1", Password: "p1"})
	req := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	req2 := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	rec2 := httptest.NewRecorder()
	e.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusConflict, rec2.Code)
}

func TestLogin_Unauthorized(t *testing.T) {
	settings.AppConfig = &settings.Config{SecretKeyForJWT: "test-secret"}
	gdb := newTestDB(t)
	e := newTestEcho(gdb)

	// user exists
	regBody, _ := json.Marshal(Credentials{Login: "u1", Password: "p1"})
	regReq := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	e.ServeHTTP(regRec, regReq)
	require.Equal(t, http.StatusOK, regRec.Code)

	// wrong password
	loginBody, _ := json.Marshal(Credentials{Login: "u1", Password: "wrong"})
	loginReq := httptest.NewRequest(http.MethodPost, "/api/user/login", bytes.NewReader(loginBody))
	loginReq.Header.Set("Content-Type", "application/json")
	loginRec := httptest.NewRecorder()
	e.ServeHTTP(loginRec, loginReq)
	require.Equal(t, http.StatusUnauthorized, loginRec.Code)
}

func TestOrders_Unauthorized(t *testing.T) {
	settings.AppConfig = &settings.Config{SecretKeyForJWT: "test-secret"}
	gdb := newTestDB(t)
	e := newTestEcho(gdb)

	req := httptest.NewRequest(http.MethodGet, "/api/user/orders", nil)
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)
}

func TestUploadOrder_InvalidNumber_422(t *testing.T) {
	settings.AppConfig = &settings.Config{SecretKeyForJWT: "test-secret"}
	gdb := newTestDB(t)
	e := newTestEcho(gdb)

	// register, get cookie
	regBody, _ := json.Marshal(Credentials{Login: "u1", Password: "p1"})
	regReq := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	e.ServeHTTP(regRec, regReq)
	sessionVal := extractSessionCookieValue(regRec.Header().Get("Set-Cookie"))
	require.NotEmpty(t, sessionVal)

	req := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader("1234567890")) // invalid Luhn
	req.Header.Set("Content-Type", "text/plain")
	req.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	rec := httptest.NewRecorder()
	e.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnprocessableEntity, rec.Code)
}

func TestUploadOrder_ConflictOtherUser_409(t *testing.T) {
	settings.AppConfig = &settings.Config{SecretKeyForJWT: "test-secret"}
	gdb := newTestDB(t)
	e := newTestEcho(gdb)

	// register user1
	regBody1, _ := json.Marshal(Credentials{Login: "u1", Password: "p1"})
	regReq1 := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(regBody1))
	regReq1.Header.Set("Content-Type", "application/json")
	regRec1 := httptest.NewRecorder()
	e.ServeHTTP(regRec1, regReq1)
	sess1 := extractSessionCookieValue(regRec1.Header().Get("Set-Cookie"))
	require.NotEmpty(t, sess1)

	// register user2
	regBody2, _ := json.Marshal(Credentials{Login: "u2", Password: "p2"})
	regReq2 := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(regBody2))
	regReq2.Header.Set("Content-Type", "application/json")
	regRec2 := httptest.NewRecorder()
	e.ServeHTTP(regRec2, regReq2)
	sess2 := extractSessionCookieValue(regRec2.Header().Get("Set-Cookie"))
	require.NotEmpty(t, sess2)

	orderNumber := "79927398713"

	// user1 uploads
	upReq1 := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(orderNumber))
	upReq1.Header.Set("Content-Type", "text/plain")
	upReq1.AddCookie(&http.Cookie{Name: "session", Value: sess1})
	upRec1 := httptest.NewRecorder()
	e.ServeHTTP(upRec1, upReq1)
	require.Equal(t, http.StatusAccepted, upRec1.Code)

	// user2 uploads same -> 409
	upReq2 := httptest.NewRequest(http.MethodPost, "/api/user/orders", strings.NewReader(orderNumber))
	upReq2.Header.Set("Content-Type", "text/plain")
	upReq2.AddCookie(&http.Cookie{Name: "session", Value: sess2})
	upRec2 := httptest.NewRecorder()
	e.ServeHTTP(upRec2, upReq2)
	require.Equal(t, http.StatusConflict, upRec2.Code)
}

func TestWithdraw_InvalidOrder_422(t *testing.T) {
	settings.AppConfig = &settings.Config{SecretKeyForJWT: "test-secret"}
	gdb := newTestDB(t)
	e := newTestEcho(gdb)

	// register, get cookie
	regBody, _ := json.Marshal(Credentials{Login: "u1", Password: "p1"})
	regReq := httptest.NewRequest(http.MethodPost, "/api/user/register", bytes.NewReader(regBody))
	regReq.Header.Set("Content-Type", "application/json")
	regRec := httptest.NewRecorder()
	e.ServeHTTP(regRec, regReq)
	sessionVal := extractSessionCookieValue(regRec.Header().Get("Set-Cookie"))
	require.NotEmpty(t, sessionVal)

	withdrawBody, _ := json.Marshal(WithdrawRequest{Order: "abc", Sum: 10})
	wdReq := httptest.NewRequest(http.MethodPost, "/api/user/balance/withdraw", bytes.NewReader(withdrawBody))
	wdReq.Header.Set("Content-Type", "application/json")
	wdReq.AddCookie(&http.Cookie{Name: "session", Value: sessionVal})
	wdRec := httptest.NewRecorder()
	e.ServeHTTP(wdRec, wdReq)
	require.Equal(t, http.StatusUnprocessableEntity, wdRec.Code)
}

