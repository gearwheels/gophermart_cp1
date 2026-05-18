// Пакет loggermid предоставляет Echo middleware, который логирует каждый
// HTTP-запрос и соответствующий ответ через структурированный slog-логгер.
// Каждая запись журнала содержит URI, метод, код ответа, размер ответа,
// задержку, удалённый IP, User-Agent и Referer.
package loggermid

import (
	"log/slog"
	"time"

	"github.com/labstack/echo/v4"
)

// RequestLogger возвращает Echo middleware, который логирует каждый входящий
// запрос и ответ на него с помощью предоставленного структурированного
// логгера. На каждый запрос эмитируются две записи slog: "HTTP запрос"
// (после возврата обработчика) и "HTTP ответ" (то же событие, вынесено
// отдельно для удобной фильтрации логов).
func RequestLogger(logger *slog.Logger) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			start := time.Now()

			err := next(c)

			duration := time.Since(start)

			res := c.Response()
			status := res.Status
			size := res.Size

			logger.LogAttrs(c.Request().Context(), slog.LevelInfo, "HTTP запрос",
				slog.String("URI", c.Request().RequestURI),
				slog.String("метод", c.Request().Method),
				slog.String("параметры", c.Request().URL.RawQuery),
				slog.Int("статус", status),
				slog.Int64("размер", size),
				slog.String("длительность", duration.String()),
				slog.String("remote_ip", c.RealIP()),
				slog.String("user_agent", c.Request().UserAgent()),
				slog.String("referer", c.Request().Referer()),
				slog.String("протокол", c.Request().Proto),
			)

			logger.LogAttrs(c.Request().Context(), slog.LevelInfo, "HTTP ответ",
				slog.Int("статус", status),
				slog.Int64("размер", size),
			)

			return err
		}
	}
}
