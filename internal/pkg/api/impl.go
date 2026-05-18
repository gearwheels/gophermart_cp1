// Пакет api реализует HTTP-обработчики Гофермарта, сгенерированные из
// спецификации OpenAPI. Тип Server удовлетворяет интерфейсу StrictServerInterface,
// сгенерированному oapi-codegen, и связывает аутентификацию, валидацию входных
// данных и доступ к базе данных для каждого эндпоинта.
package api

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	authmid "github.com/gearwheels/gophermart_cp1/internal/middleware/auth"
	"github.com/gearwheels/gophermart_cp1/internal/model"
	repopg "github.com/gearwheels/gophermart_cp1/internal/repo_pg"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Server хранит общие зависимости (соединение с БД, репозитории) и реализует
// все HTTP-обработчики, описанные в спецификации OpenAPI.
type Server struct {
	db          *gorm.DB
	users       *repopg.UserRepository
	orders      *repopg.OrderRepository
	withdrawals *repopg.WithdrawalRepository
}

// NewServer создаёт Server и инициализирует все репозитории на основе db.
func NewServer(db *gorm.DB) *Server {
	return &Server{
		db:          db,
		users:       repopg.NewUserRepository(db),
		orders:      repopg.NewOrderRepository(db),
		withdrawals: repopg.NewWithdrawalRepository(db),
	}
}

// GetBalance обрабатывает GET /api/user/balance.
// Возвращает текущий баланс баллов аутентифицированного пользователя и
// суммарное количество списанных за всё время баллов.
// Ответы: 200 с JSON-объектом Balance, 401 при отсутствии аутентификации,
// 500 при внутренней ошибке.
func (s *Server) GetBalance(ctx echo.Context) error {
	userID, ok := authmid.GetUserID(ctx)
	if !ok {
		return ctx.NoContent(http.StatusUnauthorized)
	}

	var u model.User
	if err := s.db.WithContext(ctx.Request().Context()).First(&u, userID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.NoContent(http.StatusUnauthorized)
		}
		return ctx.NoContent(http.StatusInternalServerError)
	}

	return ctx.JSON(http.StatusOK, Balance{
		Current:   float32(u.CurrentBalance),
		Withdrawn: float32(u.Withdrawn),
	})
}

// WithdrawPoints обрабатывает POST /api/user/balance/withdraw.
// Списывает запрошенное количество баллов с баланса аутентифицированного
// пользователя и фиксирует операцию списания. Операция выполняется в
// транзакции с блокировкой строки, предотвращающей параллельное
// превышение баланса.
// Ответы: 200 при успехе, 402 при недостатке средств, 422 при неверном
// номере заказа, 401 при отсутствии аутентификации, 500 при внутренней ошибке.
func (s *Server) WithdrawPoints(ctx echo.Context) error {
	userID, ok := authmid.GetUserID(ctx)
	if !ok {
		return ctx.NoContent(http.StatusUnauthorized)
	}

	var req WithdrawRequest
	if err := ctx.Bind(&req); err != nil {
		return ctx.NoContent(http.StatusBadRequest)
	}
	orderNumber := normalizeOrderNumber(req.Order)
	if orderNumber == "" || !isDigitsOnly(orderNumber) || !luhnValid(orderNumber) {
		return ctx.NoContent(http.StatusUnprocessableEntity)
	}
	if req.Sum <= 0 {
		return ctx.NoContent(http.StatusBadRequest)
	}

	now := time.Now().UTC()
	err := s.db.WithContext(ctx.Request().Context()).Transaction(func(tx *gorm.DB) error {
		var u model.User
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, userID).Error; err != nil {
			return err
		}
		if u.CurrentBalance < float64(req.Sum) {
			return echo.NewHTTPError(http.StatusPaymentRequired)
		}

		w := &model.Withdrawal{
			UserID:      userID,
			OrderNumber: orderNumber,
			Sum:         float64(req.Sum),
			ProcessedAt: now,
		}
		if err := tx.Create(w).Error; err != nil {
			return err
		}

		u.CurrentBalance -= float64(req.Sum)
		u.Withdrawn += float64(req.Sum)
		if err := tx.Model(&model.User{}).Where("id = ?", userID).Updates(map[string]any{
			"current_balance": u.CurrentBalance,
			"withdrawn":       u.Withdrawn,
			"updated_at":      now,
		}).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		if httpErr, ok := err.(*echo.HTTPError); ok && httpErr.Code == http.StatusPaymentRequired {
			return ctx.NoContent(http.StatusPaymentRequired)
		}
		return ctx.NoContent(http.StatusInternalServerError)
	}

	return ctx.NoContent(http.StatusOK)
}

// LoginUser обрабатывает POST /api/user/login.
// Проверяет пару логин/пароль через bcrypt и при успехе устанавливает
// подписанный HMAC-SHA256 сессионный cookie.
// Ответы: 200 при успехе, 400 при пустых данных, 401 при неверных
// учётных данных, 500 при внутренней ошибке.
func (s *Server) LoginUser(ctx echo.Context) error {
	var creds Credentials
	if err := ctx.Bind(&creds); err != nil {
		return ctx.NoContent(http.StatusBadRequest)
	}
	creds.Login = strings.TrimSpace(creds.Login)
	if creds.Login == "" || creds.Password == "" {
		return ctx.NoContent(http.StatusBadRequest)
	}

	u, err := s.users.GetByLogin(ctx.Request().Context(), creds.Login)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.NoContent(http.StatusUnauthorized)
		}
		return ctx.NoContent(http.StatusInternalServerError)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(creds.Password)); err != nil {
		return ctx.NoContent(http.StatusUnauthorized)
	}

	authmid.SetSessionCookie(ctx, u.ID)
	return ctx.NoContent(http.StatusOK)
}

// GetOrders обрабатывает GET /api/user/orders.
// Возвращает все заказы аутентифицированного пользователя, отсортированные
// от новых к старым. Поле accrual включается только для заказов в статусе
// PROCESSED.
// Ответы: 200 с JSON-массивом, 204 при отсутствии заказов, 401 при
// отсутствии аутентификации, 500 при внутренней ошибке.
func (s *Server) GetOrders(ctx echo.Context) error {
	userID, ok := authmid.GetUserID(ctx)
	if !ok {
		return ctx.NoContent(http.StatusUnauthorized)
	}

	orders, err := s.orders.GetByUserID(ctx.Request().Context(), userID)
	if err != nil {
		return ctx.NoContent(http.StatusInternalServerError)
	}
	if len(orders) == 0 {
		return ctx.NoContent(http.StatusNoContent)
	}

	resp := make([]OrderInfo, 0, len(orders))
	for _, o := range orders {
		var accrual *float32
		if o.Status == model.OrderStatusProcessed && o.Accrual != nil {
			v := float32(*o.Accrual)
			accrual = &v
		}
		resp = append(resp, OrderInfo{
			Number:     o.Number,
			Status:     OrderInfoStatus(o.Status),
			Accrual:    accrual,
			UploadedAt: o.UploadedAt,
		})
	}

	return ctx.JSON(http.StatusOK, resp)
}

// UploadOrder обрабатывает POST /api/user/orders.
// Принимает номер заказа в виде plain-text, валидирует его (только цифры +
// алгоритм Луна) и создаёт запись заказа в статусе NEW для последующей
// обработки системой начисления. Повторная загрузка тем же пользователем
// возвращает 200; дублирование от другого пользователя — 409.
// Ответы: 202 для нового заказа, 200 если уже загружен этим пользователем,
// 400 при пустом теле, 422 при неверном номере, 409 при конфликте с другим
// пользователем, 401 при отсутствии аутентификации, 500 при внутренней ошибке.
func (s *Server) UploadOrder(ctx echo.Context) error {
	userID, ok := authmid.GetUserID(ctx)
	if !ok {
		return ctx.NoContent(http.StatusUnauthorized)
	}

	body, err := io.ReadAll(ctx.Request().Body)
	if err != nil {
		return ctx.NoContent(http.StatusBadRequest)
	}
	orderNumber := normalizeOrderNumber(string(body))
	if orderNumber == "" {
		return ctx.NoContent(http.StatusBadRequest)
	}
	if !isDigitsOnly(orderNumber) {
		return ctx.NoContent(http.StatusUnprocessableEntity)
	}
	if !luhnValid(orderNumber) {
		return ctx.NoContent(http.StatusUnprocessableEntity)
	}

	var existing model.Order
	err = s.db.WithContext(ctx.Request().Context()).First(&existing, "number = ?", orderNumber).Error
	if err == nil {
		if existing.UserID == userID {
			return ctx.NoContent(http.StatusOK)
		}
		return ctx.NoContent(http.StatusConflict)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.NoContent(http.StatusInternalServerError)
	}

	o := &model.Order{
		Number:     orderNumber,
		UserID:     userID,
		Status:     model.OrderStatusNew,
		UploadedAt: time.Now().UTC(),
	}
	if err := s.db.WithContext(ctx.Request().Context()).Create(o).Error; err != nil {
		// если параллельно создали — проверим кому принадлежит
		var check model.Order
		if err2 := s.db.WithContext(ctx.Request().Context()).First(&check, "number = ?", orderNumber).Error; err2 == nil {
			if check.UserID == userID {
				return ctx.NoContent(http.StatusOK)
			}
			return ctx.NoContent(http.StatusConflict)
		}
		return ctx.NoContent(http.StatusInternalServerError)
	}

	return ctx.NoContent(http.StatusAccepted)
}

// RegisterUser обрабатывает POST /api/user/register.
// Создаёт новую учётную запись с bcrypt-хэшем пароля и немедленно
// аутентифицирует пользователя, устанавливая сессионный cookie — отдельный
// шаг входа после успешной регистрации не требуется.
// Ответы: 200 при успехе, 400 при пустых данных, 409 если логин занят,
// 500 при внутренней ошибке.
func (s *Server) RegisterUser(ctx echo.Context) error {
	var creds Credentials
	if err := ctx.Bind(&creds); err != nil {
		return ctx.NoContent(http.StatusBadRequest)
	}
	creds.Login = strings.TrimSpace(creds.Login)
	if creds.Login == "" || creds.Password == "" {
		return ctx.NoContent(http.StatusBadRequest)
	}

	_, err := s.users.GetByLogin(ctx.Request().Context(), creds.Login)
	if err == nil {
		return ctx.NoContent(http.StatusConflict)
	}
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.NoContent(http.StatusInternalServerError)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(creds.Password), bcrypt.DefaultCost)
	if err != nil {
		return ctx.NoContent(http.StatusInternalServerError)
	}

	u := &model.User{
		Login:        creds.Login,
		PasswordHash: string(hash),
	}
	if err := s.db.WithContext(ctx.Request().Context()).Create(u).Error; err != nil {
		// на случай гонки при одновременной регистрации с одинаковым логином
		if strings.Contains(strings.ToLower(err.Error()), "duplicate") || strings.Contains(strings.ToLower(err.Error()), "unique") {
			return ctx.NoContent(http.StatusConflict)
		}
		return ctx.NoContent(http.StatusInternalServerError)
	}

	authmid.SetSessionCookie(ctx, u.ID)
	return ctx.NoContent(http.StatusOK)
}

// GetWithdrawals обрабатывает GET /api/user/withdrawals.
// Возвращает полную историю списаний аутентифицированного пользователя,
// отсортированную от новых к старым по полю processed_at.
// Ответы: 200 с JSON-массивом, 204 при отсутствии списаний, 401 при
// отсутствии аутентификации, 500 при внутренней ошибке.
func (s *Server) GetWithdrawals(ctx echo.Context) error {
	userID, ok := authmid.GetUserID(ctx)
	if !ok {
		return ctx.NoContent(http.StatusUnauthorized)
	}

	ws, err := s.withdrawals.GetByUserID(ctx.Request().Context(), userID)
	if err != nil {
		return ctx.NoContent(http.StatusInternalServerError)
	}
	if len(ws) == 0 {
		return ctx.NoContent(http.StatusNoContent)
	}

	resp := make([]Withdrawal, 0, len(ws))
	for _, w := range ws {
		resp = append(resp, Withdrawal{
			Order:       w.OrderNumber,
			Sum:         float32(w.Sum),
			ProcessedAt: w.ProcessedAt,
		})
	}

	return ctx.JSON(http.StatusOK, resp)
}

func ptrString(s string) *string { return &s }

var errUnauthorized = errors.New("не авторизован")

// normalizeOrderNumber обрезает начальные и конечные пробелы в строке номера
// заказа, полученной из тела запроса.
func normalizeOrderNumber(v string) string {
	return strings.TrimSpace(v)
}

// isDigitsOnly сообщает, состоит ли s исключительно из десятичных ASCII-цифр
// и не является ли пустой строкой.
func isDigitsOnly(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// luhnValid сообщает, проходит ли number проверку по алгоритму Луна,
// используемому для валидации номеров заказов.
func luhnValid(number string) bool {
	// алгоритм Луна для строки цифр
	sum := 0
	alt := false
	for i := len(number) - 1; i >= 0; i-- {
		d := int(number[i] - '0')
		if alt {
			d *= 2
			if d > 9 {
				d -= 9
			}
		}
		sum += d
		alt = !alt
	}
	return sum%10 == 0
}
