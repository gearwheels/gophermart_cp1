package api

import (
	// "context"
	"net/http"
	"github.com/labstack/echo/v4"  
)

type Server struct{}

func NewServer() Server {
    return Server{}
}

// func (Server) ListDevices(ctx context.Context, request ListDevicesRequestObject) (ListDevicesResponseObject, error) {
//     // здесь будет реальная логика
//     return ListDevices200JSONResponse{}, nil
// }

// func (Server) GetDevice(ctx context.Context, request GetDeviceRequestObject) (GetDeviceResponseObject, error) {
//     // здесь будет реальная логика
//     return GetDevice200JSONResponse{}, nil
// }


// Get current loyalty points balance and total withdrawn
// (GET /api/user/balance)
func (Server) GetBalance(ctx echo.Context) error{
	resp := GetBalance200JSONResponse{
		Current: 100,
		Withdrawn: 50,
	}

	return ctx.JSON(http.StatusOK, resp)
}

// Withdraw points to pay for a new order
// (POST /api/user/balance/withdraw)
func (Server) WithdrawPoints(ctx echo.Context) error {
	return ctx.NoContent(http.StatusOK)
}

// Authenticate user
// (POST /api/user/login)
func (Server) LoginUser(ctx echo.Context) error{
	resp := LoginUser200Response{	}	

	return ctx.JSON(http.StatusOK, resp)
}
// Get list of uploaded orders with statuses and accruals
// (GET /api/user/orders)
func (Server) GetOrders(ctx echo.Context) error{
	resp := GetOrders200JSONResponse{}

	return ctx.JSON(http.StatusOK, resp)
}
// Upload an order number for processing
// (POST /api/user/orders)
func (Server) UploadOrder(ctx echo.Context) error{
	resp := UploadOrder200Response{}
	
	return ctx.JSON(http.StatusOK, resp)
}
// Register a new user
// (POST /api/user/register)
func (Server) RegisterUser(ctx echo.Context) error{
	resp := RegisterUser200Response{}

	return ctx.JSON(http.StatusOK, resp)
}
// Get withdrawal history
// (GET /api/user/withdrawals)
func (Server) GetWithdrawals(ctx echo.Context) error{
	resp := GetWithdrawals200JSONResponse{}

	return ctx.JSON(http.StatusOK, resp)
}
