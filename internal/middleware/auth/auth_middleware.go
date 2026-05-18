// Пакет auth предоставляет Echo middleware и вспомогательные функции для
// аутентификации на основе cookie-сессий. Сессии представляют собой
// HMAC-SHA256-подписанные строки вида "userID:подпись", хранящиеся в
// HttpOnly cookie с именем "session". Middleware прозрачно разрешает
// неаутентифицированный доступ к эндпоинтам регистрации и входа, а для
// всех остальных маршрутов /api/user/* требует аутентификации.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	settings "github.com/gearwheels/gophermart_cp1/internal/config"
	"github.com/labstack/echo/v4"
	"gorm.io/gorm"
)

const (
	cookieName   = "session"
	cookieMaxAge = 30 * 24 * time.Hour
)

// signData создаёт HMAC-SHA256 подпись для данных.
func signData(data string) string {
	h := hmac.New(sha256.New, []byte(settings.AppConfig.SecretKeyForJWT))
	h.Write([]byte(data))
	return hex.EncodeToString(h.Sum(nil))
}

// ErrInvalidCookieFormat возвращается, когда значение сессионного cookie
// не удаётся разбить на ожидаемый формат "userID:подпись".
var ErrInvalidCookieFormat = errors.New("неверный формат cookie: ожидается \"userID:подпись\"")

// ErrInvalidSignature возвращается, когда HMAC-подпись в сессионном cookie
// не совпадает с вычисленной для данного userID.
var ErrInvalidSignature = errors.New("недействительная подпись cookie")

// validateCookie проверяет cookie и возвращает userID при верной подписи.
func validateCookie(cookieValue string) (int64, error) {
	parts := strings.SplitN(cookieValue, ":", 2)
	if len(parts) != 2 {
		return 0, ErrInvalidCookieFormat
	}
	userID, signature := parts[0], parts[1]
	expectedSign := signData(userID)
	if !hmac.Equal([]byte(signature), []byte(expectedSign)) {
		return 0, ErrInvalidSignature
	}
	id64, err := parseInt64(userID)
	if err != nil {
		return 0, ErrInvalidCookieFormat
	}
	return id64, nil
}

// BuildCookieValue возвращает сырое значение cookie "userID:HMAC" для
// указанного userID. Экспортируется, чтобы тесты могли формировать
// корректные токены сессии без HTTP round-trip.
func BuildCookieValue(userID int64) string {
	userIDStr := fmt.Sprintf("%d", userID)
	signature := signData(userIDStr)
	return userIDStr + ":" + signature
}

// SetSessionCookie записывает в ответ подписанный сессионный cookie для
// указанного userID. Cookie является HttpOnly, SameSite=Strict и действителен
// 30 дней.
func SetSessionCookie(ctx echo.Context, userID int64) {
	ctx.SetCookie(&http.Cookie{
		Name:     cookieName,
		Value:    BuildCookieValue(userID),
		Path:     "/",
		MaxAge:   int(cookieMaxAge.Seconds()),
		HttpOnly: true,
		Secure:   false,
		SameSite: http.SameSiteStrictMode,
	})
}

const userIDContextKey = "userID"

// GetUserID извлекает ID аутентифицированного пользователя из контекста Echo.
// Возвращает (id, true), если middleware уже проверил сессию, или (0, false),
// если значение отсутствует (например, на публичных маршрутах).
func GetUserID(ctx echo.Context) (int64, bool) {
	v := ctx.Get(userIDContextKey)
	id, ok := v.(int64)
	return id, ok
}

// Middleware возвращает Echo middleware, который обеспечивает аутентификацию
// на всех маршрутах /api/user/*, кроме /api/user/register и /api/user/login.
// Корректный токен сессии должен присутствовать либо в cookie "session", либо
// в заголовке Authorization: Bearer. При успехе userID сохраняется в контексте
// Echo под ключом "userID" для дальнейших обработчиков.
func Middleware(db *gorm.DB) echo.MiddlewareFunc {
	_ = db // db зарезервирован для будущей проверки существования пользователя
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(ctx echo.Context) error {
			p := ctx.Path()
			if p == "" {
				p = ctx.Request().URL.Path
			}
			if p == "/api/user/register" || p == "/api/user/login" {
				return next(ctx)
			}

			if !strings.HasPrefix(p, "/api/user/") {
				return next(ctx)
			}

			token := ""
			if c, err := ctx.Cookie(cookieName); err == nil && c != nil {
				token = c.Value
			}
			if token == "" {
				authz := ctx.Request().Header.Get("Authorization")
				if strings.HasPrefix(authz, "Bearer ") {
					token = strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
				}
			}
			if token == "" {
				return echo.NewHTTPError(401)
			}

			userID, err := validateCookie(token)
			if err != nil {
				return echo.NewHTTPError(401)
			}

			ctx.Set(userIDContextKey, userID)
			return next(ctx)
		}
	}
}

func parseInt64(s string) (int64, error) {
	var v int64
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, ErrInvalidCookieFormat
		}
		v = v*10 + int64(r-'0')
	}
	if v <= 0 {
		return 0, ErrInvalidCookieFormat
	}
	return v, nil
}
