// Пакет accrual предоставляет HTTP-клиент для взаимодействия с внешней
// системой начисления баллов лояльности. Клиент преобразует HTTP-ответы
// (200, 204, 429, 5xx) в типизированные ошибки Go, избавляя вызывающий код
// от необходимости проверять статус-коды напрямую.
package accrual

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Status представляет состояние расчёта, возвращаемое системой начисления.
type Status string

const (
	// StatusRegistered означает, что заказ зарегистрирован в системе начисления,
	// но расчёт ещё не начат.
	StatusRegistered Status = "REGISTERED"
	// StatusInvalid означает, что система начисления отказала в расчёте;
	// вознаграждение начислено не будет.
	StatusInvalid Status = "INVALID"
	// StatusProcessing означает, что расчёт вознаграждения выполняется.
	StatusProcessing Status = "PROCESSING"
	// StatusProcessed означает, что расчёт завершён и поле Accrual содержит
	// итоговую сумму начисления.
	StatusProcessed Status = "PROCESSED"
)

// OrderInfo — тело ответа системы начисления при успешном запросе заказа
// (HTTP 200).
type OrderInfo struct {
	// Order — номер заказа, возвращённый системой начисления.
	Order string `json:"order"`
	// Status — текущее состояние расчёта.
	Status Status `json:"status"`
	// Accrual — количество начисленных баллов лояльности. Поле отсутствует
	// в ответе, если начисление не предусмотрено.
	Accrual *float64 `json:"accrual,omitempty"`
}

// ErrNotRegistered возвращается из GetOrder, когда система начисления отвечает
// HTTP 204 — заказ ей ещё не известен.
var ErrNotRegistered = errors.New("заказ не зарегистрирован в системе начисления")

// ErrTooMany — сигнальная ошибка, оборачиваемая внутри TooManyError.
var ErrTooMany = errors.New("превышено число запросов")

// TooManyError возвращается из GetOrder, когда система начисления отвечает
// HTTP 429. RetryAfter содержит задержку, разобранную из заголовка Retry-After.
type TooManyError struct {
	RetryAfter time.Duration
}

// Error реализует интерфейс error.
func (e *TooManyError) Error() string {
	return fmt.Sprintf("превышено число запросов, повтор через %s", e.RetryAfter)
}

// Client — HTTP-клиент для системы начисления. Создаётся через NewClient;
// клиент с пустым базовым URL считается отключённым и возвращает ошибку
// на каждый вызов.
type Client struct {
	baseURL string
	hc      *http.Client
}

// NewClient создаёт новый Client с указанным baseURL. Завершающие слэши
// обрезаются автоматически. Передача пустой строки создаёт отключённый
// клиент (Enabled возвращает false).
func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		hc: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Enabled сообщает, задан ли непустой базовый URL. Воркер начисления
// пропускает обработку, если Enabled возвращает false.
func (c *Client) Enabled() bool { return c.baseURL != "" }

// GetOrder запрашивает у системы начисления статус расчёта для указанного
// номера заказа.
//
// Возможные возвращаемые значения:
//   - (*OrderInfo, nil)        — HTTP 200, заказ найден; проверьте OrderInfo.Status.
//   - (nil, ErrNotRegistered)  — HTTP 204, система ещё не знает о заказе.
//   - (nil, *TooManyError)     — HTTP 429, необходимо выдержать паузу.
//   - (nil, error)             — любая иная ошибка (сеть, декодирование и т.д.).
func (c *Client) GetOrder(ctx context.Context, number string) (*OrderInfo, error) {
	if c.baseURL == "" {
		return nil, errors.New("клиент начисления отключён: не задан базовый URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/orders/"+number, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		var oi OrderInfo
		if err := json.NewDecoder(resp.Body).Decode(&oi); err != nil {
			return nil, err
		}
		return &oi, nil
	case http.StatusNoContent:
		return nil, ErrNotRegistered
	case http.StatusTooManyRequests:
		ra := resp.Header.Get("Retry-After")
		secs, _ := strconv.Atoi(strings.TrimSpace(ra))
		if secs <= 0 {
			secs = 60
		}
		return nil, &TooManyError{RetryAfter: time.Duration(secs) * time.Second}
	default:
		return nil, fmt.Errorf("неожиданный статус от системы начисления: %d", resp.StatusCode)
	}
}
