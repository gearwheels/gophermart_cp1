package loggermid


import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
)

// // RequestLogger создает middleware для логирования HTTP-запросов
// func RequestLogger(logger *slog.Logger) func(next http.Handler) http.Handler {
// 	return func(next http.Handler) http.Handler {
// 		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
// 			// Засекаем время начала обработки
// 			start := time.Now()

// 			// Создаем ResponseWriter для отслеживания статуса и размера ответа
// 			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

// 			// Обрабатываем запрос
// 			next.ServeHTTP(ww, r)

// 			// Время обработки
// 			duration := time.Since(start)

// 			// Логируем информацию о запросе
// 			logger.LogAttrs(r.Context(), slog.LevelInfo, "HTTP request",
// 				slog.String("URI", r.RequestURI),
// 				slog.String("method", r.Method),
// 				slog.String("query", r.URL.RawQuery),
// 				slog.Int("status", ww.Status()),
// 				slog.Int("size", ww.BytesWritten()),
// 				slog.String("duration", duration.String()),
// 				slog.String("remote_ip", r.RemoteAddr),
// 				slog.String("user_agent", r.UserAgent()),
// 				slog.String("referer", r.Referer()),
// 				slog.String("protocol", r.Proto),
// 			)

// 			logger.LogAttrs(r.Context(), slog.LevelInfo, "HTTP response",
// 				slog.Int("status", ww.Status()),
// 				slog.Int("size", ww.BytesWritten()),
// 			)
// 		})
// 	}
// }

// RequestLogger создает middleware для логирования HTTP-запросов в Echo
func RequestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			// Выполняем следующий обработчик
			err := next(c)

			// Время обработки
			duration := time.Since(start)

			// Данные ответа
			res := c.Response()
			status := res.Status
			size := res.Size

			// Логируем информацию о запросе
			logger.LogAttrs(c.Request().Context(), slog.LevelInfo, "HTTP request",
				slog.String("URI", c.Request().RequestURI),
				slog.String("method", c.Request().Method),
				slog.String("query", c.Request().URL.RawQuery),
				slog.Int("status", status),
				slog.Int64("size", size),
				slog.String("duration", duration.String()),
				slog.String("remote_ip", c.RealIP()),
				slog.String("user_agent", c.Request().UserAgent()),
				slog.String("referer", c.Request().Referer()),
				slog.String("protocol", c.Request().Proto),
			)

			// Логируем ответ (можно объединить с предыдущим)
			logger.LogAttrs(c.Request().Context(), slog.LevelInfo, "HTTP response",
				slog.Int("status", status),
				slog.Int64("size", size),
			)

			return err
		}
	}
}


