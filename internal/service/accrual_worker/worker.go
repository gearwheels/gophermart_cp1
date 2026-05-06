// Пакет accrual_worker реализует фоновое задание, которое опрашивает внешнюю
// систему начисления по ожидающим заказам и атомарно начисляет вознаграждение
// на счета пользователей.
package accrual_worker

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/gearwheels/gophermart_cp1/internal/accrual"
	"github.com/gearwheels/gophermart_cp1/internal/model"
	repopg "github.com/gearwheels/gophermart_cp1/internal/repo_pg"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Worker опрашивает внешнюю систему начисления по заказам в статусе NEW или
// PROCESSING и применяет рассчитанные вознаграждения к балансам пользователей.
// Каждое вознаграждение начисляется ровно один раз благодаря флагу
// accrual_applied и пессимистичной блокировке строк внутри транзакции.
type Worker struct {
	db     *gorm.DB
	orders *repopg.OrderRepository
	client *accrual.Client
	limit  int
}

// New создаёт Worker, который выбирает не более limit заказов за одну итерацию
// опроса. Если limit ≤ 0, используется значение по умолчанию 10. Передача
// accrual.Client с Enabled() == false создаёт воркер-заглушку (Run вернётся
// немедленно).
func New(db *gorm.DB, client *accrual.Client, limit int) *Worker {
	if limit <= 0 {
		limit = 10
	}
	return &Worker{
		db:     db,
		orders: repopg.NewOrderRepository(db),
		client: client,
		limit:  limit,
	}
}

// Run запускает цикл опроса и блокирует выполнение до отмены ctx. Метод
// должен вызываться в отдельной горутине. Если клиент начисления отключён
// (ACCRUAL_SYSTEM_ADDRESS не задан), Run логирует сообщение и немедленно
// завершает работу.
func (w *Worker) Run(ctx context.Context) {
	if w.client == nil || !w.client.Enabled() {
		slog.Info("воркер начисления отключён (адрес системы не задан)")
		return
	}

	backoff := 500 * time.Millisecond
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		orders, err := w.orders.GetProcessingBatch(ctx, w.limit)
		if err != nil {
			slog.Error("воркер начисления: ошибка загрузки заказов", slog.String("err", err.Error()))
			time.Sleep(2 * time.Second)
			continue
		}
		if len(orders) == 0 {
			time.Sleep(800 * time.Millisecond)
			continue
		}

		for _, o := range orders {
			select {
			case <-ctx.Done():
				return
			default:
			}

			info, err := w.client.GetOrder(ctx, o.Number)
			if err != nil {
				var tme *accrual.TooManyError
				if errors.As(err, &tme) {
					time.Sleep(tme.RetryAfter)
					continue
				}
				if errors.Is(err, accrual.ErrNotRegistered) {
					// внешний сервис ещё не знает о заказе → оставляем NEW
					continue
				}

				time.Sleep(backoff)
				if backoff < 5*time.Second {
					backoff *= 2
				}
				continue
			}
			backoff = 500 * time.Millisecond

			_ = w.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
				var cur model.Order
				if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&cur, "number = ?", o.Number).Error; err != nil {
					return err
				}

				switch info.Status {
				case accrual.StatusRegistered:
					// Система начисления знает о заказе, но расчёт ещё не начат;
					// оставляем статус NEW, чтобы заказ остался в очереди опроса.
				case accrual.StatusProcessing:
					cur.Status = model.OrderStatusProcessing
				case accrual.StatusInvalid:
					cur.Status = model.OrderStatusInvalid
				case accrual.StatusProcessed:
					cur.Status = model.OrderStatusProcessed
					cur.Accrual = info.Accrual
				default:
					// неизвестный статус: оставляем без изменений
				}

				// Начисляем баллы ровно один раз
				if cur.Status == model.OrderStatusProcessed && cur.Accrual != nil && !cur.AccrualApplied && *cur.Accrual > 0 {
					var u model.User
					if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&u, cur.UserID).Error; err != nil {
						return err
					}
					u.CurrentBalance += *cur.Accrual
					if err := tx.Model(&model.User{}).Where("id = ?", u.ID).Update("current_balance", u.CurrentBalance).Error; err != nil {
						return err
					}
					cur.AccrualApplied = true
				}

				return tx.Model(&model.Order{}).Where("number = ?", cur.Number).Updates(map[string]any{
					"status":          cur.Status,
					"accrual":         cur.Accrual,
					"accrual_applied": cur.AccrualApplied,
				}).Error
			})
		}
	}
}
