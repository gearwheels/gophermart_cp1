package api

import (
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestGeneratedResponses_Visit(t *testing.T) {
	msg := "x"

	t.Run("GetBalance", func(t *testing.T) {
		rec := httptest.NewRecorder()
		require.NoError(t, GetBalance200JSONResponse{Current: 1.5, Withdrawn: 2}.VisitGetBalanceResponse(rec))
		require.Equal(t, 200, rec.Code)

		rec = httptest.NewRecorder()
		require.NoError(t, GetBalance401JSONResponse{Message: &msg}.VisitGetBalanceResponse(rec))
		require.Equal(t, 401, rec.Code)

		rec = httptest.NewRecorder()
		require.NoError(t, GetBalance500JSONResponse{Message: &msg}.VisitGetBalanceResponse(rec))
		require.Equal(t, 500, rec.Code)
	})

	t.Run("WithdrawPoints", func(t *testing.T) {
		rec := httptest.NewRecorder()
		require.NoError(t, WithdrawPoints200Response{}.VisitWithdrawPointsResponse(rec))
		require.Equal(t, 200, rec.Code)

		for _, tc := range []struct {
			code int
			fn   func(*httptest.ResponseRecorder) error
		}{
			{401, func(r *httptest.ResponseRecorder) error { return WithdrawPoints401JSONResponse{Message: &msg}.VisitWithdrawPointsResponse(r) }},
			{402, func(r *httptest.ResponseRecorder) error { return WithdrawPoints402JSONResponse{Message: &msg}.VisitWithdrawPointsResponse(r) }},
			{422, func(r *httptest.ResponseRecorder) error { return WithdrawPoints422JSONResponse{Message: &msg}.VisitWithdrawPointsResponse(r) }},
			{500, func(r *httptest.ResponseRecorder) error { return WithdrawPoints500JSONResponse{Message: &msg}.VisitWithdrawPointsResponse(r) }},
		} {
			rec := httptest.NewRecorder()
			require.NoError(t, tc.fn(rec))
			require.Equal(t, tc.code, rec.Code)
		}
	})

	t.Run("LoginUser", func(t *testing.T) {
		rec := httptest.NewRecorder()
		require.NoError(t, LoginUser200Response{Headers: LoginUser200ResponseHeaders{SetCookie: "session=a"}}.VisitLoginUserResponse(rec))
		require.Equal(t, 200, rec.Code)

		for _, tc := range []struct {
			code int
			fn   func(*httptest.ResponseRecorder) error
		}{
			{400, func(r *httptest.ResponseRecorder) error { return LoginUser400JSONResponse{Message: &msg}.VisitLoginUserResponse(r) }},
			{401, func(r *httptest.ResponseRecorder) error { return LoginUser401JSONResponse{Message: &msg}.VisitLoginUserResponse(r) }},
			{500, func(r *httptest.ResponseRecorder) error { return LoginUser500JSONResponse{Message: &msg}.VisitLoginUserResponse(r) }},
		} {
			rec := httptest.NewRecorder()
			require.NoError(t, tc.fn(rec))
			require.Equal(t, tc.code, rec.Code)
		}
	})

	t.Run("RegisterUser", func(t *testing.T) {
		rec := httptest.NewRecorder()
		require.NoError(t, RegisterUser200Response{Headers: RegisterUser200ResponseHeaders{SetCookie: "session=a"}}.VisitRegisterUserResponse(rec))
		require.Equal(t, 200, rec.Code)

		for _, tc := range []struct {
			code int
			fn   func(*httptest.ResponseRecorder) error
		}{
			{400, func(r *httptest.ResponseRecorder) error { return RegisterUser400JSONResponse{Message: &msg}.VisitRegisterUserResponse(r) }},
			{409, func(r *httptest.ResponseRecorder) error { return RegisterUser409JSONResponse{Message: &msg}.VisitRegisterUserResponse(r) }},
			{500, func(r *httptest.ResponseRecorder) error { return RegisterUser500JSONResponse{Message: &msg}.VisitRegisterUserResponse(r) }},
		} {
			rec := httptest.NewRecorder()
			require.NoError(t, tc.fn(rec))
			require.Equal(t, tc.code, rec.Code)
		}
	})

	t.Run("Orders", func(t *testing.T) {
		rec := httptest.NewRecorder()
		resp := GetOrders200JSONResponse{
			{Number: "1", Status: NEW, UploadedAt: time.Now()},
		}
		require.NoError(t, resp.VisitGetOrdersResponse(rec))
		require.Equal(t, 200, rec.Code)

		rec = httptest.NewRecorder()
		require.NoError(t, GetOrders204Response{}.VisitGetOrdersResponse(rec))
		require.Equal(t, 204, rec.Code)

		rec = httptest.NewRecorder()
		require.NoError(t, GetOrders401JSONResponse{Message: &msg}.VisitGetOrdersResponse(rec))
		require.Equal(t, 401, rec.Code)

		rec = httptest.NewRecorder()
		require.NoError(t, GetOrders500JSONResponse{Message: &msg}.VisitGetOrdersResponse(rec))
		require.Equal(t, 500, rec.Code)
	})

	t.Run("UploadOrder", func(t *testing.T) {
		for _, tc := range []struct {
			code int
			fn   func(*httptest.ResponseRecorder) error
		}{
			{200, func(r *httptest.ResponseRecorder) error { return UploadOrder200Response{}.VisitUploadOrderResponse(r) }},
			{202, func(r *httptest.ResponseRecorder) error { return UploadOrder202Response{}.VisitUploadOrderResponse(r) }},
			{400, func(r *httptest.ResponseRecorder) error { return UploadOrder400JSONResponse{Message: &msg}.VisitUploadOrderResponse(r) }},
			{401, func(r *httptest.ResponseRecorder) error { return UploadOrder401JSONResponse{Message: &msg}.VisitUploadOrderResponse(r) }},
			{409, func(r *httptest.ResponseRecorder) error { return UploadOrder409JSONResponse{Message: &msg}.VisitUploadOrderResponse(r) }},
			{422, func(r *httptest.ResponseRecorder) error { return UploadOrder422JSONResponse{Message: &msg}.VisitUploadOrderResponse(r) }},
			{500, func(r *httptest.ResponseRecorder) error { return UploadOrder500JSONResponse{Message: &msg}.VisitUploadOrderResponse(r) }},
		} {
			rec := httptest.NewRecorder()
			require.NoError(t, tc.fn(rec))
			require.Equal(t, tc.code, rec.Code)
		}
	})

	t.Run("Withdrawals", func(t *testing.T) {
		rec := httptest.NewRecorder()
		resp := GetWithdrawals200JSONResponse{
			{Order: "1", Sum: 1, ProcessedAt: time.Now()},
		}
		require.NoError(t, resp.VisitGetWithdrawalsResponse(rec))
		require.Equal(t, 200, rec.Code)

		rec = httptest.NewRecorder()
		require.NoError(t, GetWithdrawals204Response{}.VisitGetWithdrawalsResponse(rec))
		require.Equal(t, 204, rec.Code)

		rec = httptest.NewRecorder()
		require.NoError(t, GetWithdrawals401JSONResponse{Message: &msg}.VisitGetWithdrawalsResponse(rec))
		require.Equal(t, 401, rec.Code)

		rec = httptest.NewRecorder()
		require.NoError(t, GetWithdrawals500JSONResponse{Message: &msg}.VisitGetWithdrawalsResponse(rec))
		require.Equal(t, 500, rec.Code)
	})
}

