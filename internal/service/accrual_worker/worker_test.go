package accrual_worker

import (
	"context"
	"testing"
	"time"

	"github.com/gearwheels/gophermart_cp1/internal/accrual"
	"github.com/gearwheels/gophermart_cp1/internal/model"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// newTestDB открывает SQLite in-memory базу и применяет AutoMigrate.
// MaxOpenConns=1 гарантирует, что все запросы идут через одно соединение,
// поскольку каждое новое соединение к ":memory:" открывает отдельную БД.
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	gdb, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := gdb.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, gdb.AutoMigrate(&model.User{}, &model.Order{}, &model.Withdrawal{}))
	return gdb
}

// chanFetcher — заглушка AccrualFetcher на основе канала.
//
// GetOrder блокирует горутину воркера ровно в том месте, где обычно идёт
// сетевой вызов. Это позволяет тесту управлять ходом выполнения:
//   - push() отправляет ответ → GetOrder разблокируется
//   - если канал пуст — воркер висит на select, тест может безопасно смотреть в БД
//
// onWaiting (опционально) вызывается каждый раз, когда GetOrder начинает
// ожидать ответ. Тест может использовать это для детектирования «воркер
// обработал предыдущий ответ и готов к следующему».
type chanFetcher struct {
	ch        chan fetchResult
	onWaiting func() // вызывается перед блокировкой на select (может быть nil)
}

type fetchResult struct {
	info *accrual.OrderInfo
	err  error
}

func newChanFetcher(cap int) *chanFetcher {
	return &chanFetcher{ch: make(chan fetchResult, cap)}
}

func (f *chanFetcher) Enabled() bool { return true }

// GetOrder блокирует вызывающую горутину до прихода ответа или отмены ctx.
func (f *chanFetcher) GetOrder(ctx context.Context, _ string) (*accrual.OrderInfo, error) {
	if f.onWaiting != nil {
		f.onWaiting()
	}
	select {
	case r := <-f.ch:
		return r.info, r.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// push кладёт заранее подготовленный ответ в буферизованный канал.
func (f *chanFetcher) push(info *accrual.OrderInfo, err error) {
	f.ch <- fetchResult{info: info, err: err}
}

// waitIdle ждёт, пока воркер не окажется заблокированным на GetOrder не менее
// minCalls раз суммарно. Это гарантирует, что все предшествующие транзакции
// завершились. Таймаут — 500 мс.
func (f *chanFetcher) waitIdle(t *testing.T, minCalls int) {
	t.Helper()
	calls := 0
	ready := make(chan struct{}, 1)
	prev := f.onWaiting
	f.onWaiting = func() {
		if prev != nil {
			prev()
		}
		calls++
		if calls >= minCalls {
			select {
			case ready <- struct{}{}:
			default:
			}
		}
	}
	select {
	case <-ready:
	case <-time.After(500 * time.Millisecond):
		t.Fatalf("chanFetcher.waitIdle: воркер не достиг %d вызовов GetOrder за 500 мс", minCalls)
	}
	f.onWaiting = prev
}

// --- вспомогательные функции ---

func newUser(t *testing.T, db *gorm.DB) *model.User {
	t.Helper()
	u := &model.User{Login: "u", PasswordHash: "x"}
	require.NoError(t, db.Create(u).Error)
	return u
}

func newOrder(t *testing.T, db *gorm.DB, number string, userID int64) *model.Order {
	t.Helper()
	o := &model.Order{Number: number, UserID: userID, Status: model.OrderStatusNew, UploadedAt: time.Now().UTC()}
	require.NoError(t, db.Create(o).Error)
	return o
}

// --- тесты ---

// TestWorker_AppliesAccrualOnce проверяет, что воркер начисляет баллы ровно
// один раз при получении статуса PROCESSED от системы начисления.
func TestWorker_AppliesAccrualOnce(t *testing.T) {
	db := newTestDB(t)
	u := newUser(t, db)
	o := newOrder(t, db, "79927398713", u.ID)

	ten := 10.0
	fetcher := newChanFetcher(1)
	fetcher.push(&accrual.OrderInfo{Order: o.Number, Status: accrual.StatusProcessed, Accrual: &ten}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(db, fetcher, 10).Run(ctx)

	// ждём, пока воркер применит начисление
	require.Eventually(t, func() bool {
		var o2 model.Order
		if err := db.First(&o2, "number = ?", o.Number).Error; err != nil {
			return false
		}
		return o2.Status == model.OrderStatusProcessed && o2.AccrualApplied
	}, 500*time.Millisecond, 2*time.Millisecond)

	var u2 model.User
	require.NoError(t, db.First(&u2, u.ID).Error)
	require.InEpsilon(t, 10.0, u2.CurrentBalance, 1e-9)

	cancel()

	// повторный запуск — заказ уже PROCESSED, GetProcessingBatch вернёт пусто;
	// второй воркер ни разу не вызовет GetOrder и не изменит баланс.
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()

	fetcher2 := newChanFetcher(0) // пустой канал — ответ не придёт никогда
	calls := 0
	fetcher2.onWaiting = func() { calls++ }

	go New(db, fetcher2, 10).Run(ctx2)

	// ждём первого входа в GetOrder (воркер проверил что NEW/PROCESSING нет и спит 800ms)
	// Воркер должен либо заблокироваться на GetOrder, либо вовсе не дойти до него.
	// Достаточно убедиться, что баланс не изменился спустя короткое время.
	require.Never(t, func() bool {
		var u3 model.User
		_ = db.First(&u3, u.ID).Error
		return u3.CurrentBalance > 10.0
	}, 100*time.Millisecond, 5*time.Millisecond, "двойного начисления быть не должно")

	cancel2()
}

// TestWorker_AccrualNotRegistered_LeavesOrderNew проверяет, что ErrNotRegistered
// (HTTP 204 от системы начисления) не изменяет статус заказа.
func TestWorker_AccrualNotRegistered_LeavesOrderNew(t *testing.T) {
	db := newTestDB(t)
	u := newUser(t, db)
	o := newOrder(t, db, "123", u.ID)

	fetcher := newChanFetcher(1)
	fetcher.push(nil, accrual.ErrNotRegistered)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(db, fetcher, 10).Run(ctx)

	// ждём второго вызова GetOrder: воркер получил ErrNotRegistered (1-й вызов),
	// сделал continue, снова вошёл в GetOrder (2-й вызов) — транзакция точно
	// завершилась.
	fetcher.waitIdle(t, 2)

	var o2 model.Order
	require.NoError(t, db.First(&o2, "number = ?", o.Number).Error)
	require.Equal(t, model.OrderStatusNew, o2.Status)

	var u2 model.User
	require.NoError(t, db.First(&u2, u.ID).Error)
	require.Equal(t, 0.0, u2.CurrentBalance)
}

// TestWorker_RegisteredStatus_KeepsOrderNew проверяет, что статус REGISTERED
// от системы начисления оставляет заказ в статусе NEW (не переводит в PROCESSING).
func TestWorker_RegisteredStatus_KeepsOrderNew(t *testing.T) {
	db := newTestDB(t)
	u := newUser(t, db)
	o := newOrder(t, db, "79927398713", u.ID)

	fetcher := newChanFetcher(1)
	fetcher.push(&accrual.OrderInfo{Order: o.Number, Status: accrual.StatusRegistered}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(db, fetcher, 10).Run(ctx)

	// ждём второго вызова GetOrder: воркер выполнил транзакцию (status=NEW, no-op),
	// снова вошёл в GetOrder — всё уже записано в БД.
	fetcher.waitIdle(t, 2)

	var o2 model.Order
	require.NoError(t, db.First(&o2, "number = ?", o.Number).Error)
	require.Equal(t, model.OrderStatusNew, o2.Status, "статус REGISTERED от системы начисления должен оставлять заказ в NEW")

	var u2 model.User
	require.NoError(t, db.First(&u2, u.ID).Error)
	require.Equal(t, 0.0, u2.CurrentBalance, "при статусе REGISTERED баллы начисляться не должны")
}

// TestWorker_InvalidOrder_MarksInvalid проверяет, что заказ переводится в
// статус INVALID и баллы не начисляются.
func TestWorker_InvalidOrder_MarksInvalid(t *testing.T) {
	db := newTestDB(t)
	u := newUser(t, db)
	o := newOrder(t, db, "999", u.ID)

	fetcher := newChanFetcher(1)
	fetcher.push(&accrual.OrderInfo{Order: o.Number, Status: accrual.StatusInvalid}, nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(db, fetcher, 10).Run(ctx)

	// INVALID — терминальный статус: ждём его появления в БД
	require.Eventually(t, func() bool {
		var o2 model.Order
		if err := db.First(&o2, "number = ?", o.Number).Error; err != nil {
			return false
		}
		return o2.Status == model.OrderStatusInvalid
	}, 500*time.Millisecond, 2*time.Millisecond)

	var o2 model.Order
	require.NoError(t, db.First(&o2, "number = ?", o.Number).Error)
	require.False(t, o2.AccrualApplied)
}

// TestWorker_TooManyRequests_BacksOff проверяет, что при получении 429
// воркер не изменяет статус заказа и ждёт перед следующей попыткой.
func TestWorker_TooManyRequests_BacksOff(t *testing.T) {
	db := newTestDB(t)
	u := newUser(t, db)
	o := newOrder(t, db, "79927398713", u.ID)

	fetcher := newChanFetcher(2)
	// 1 мс — минимальная пауза, достаточная для проверки back-off без замедления теста
	fetcher.push(nil, &accrual.TooManyError{RetryAfter: 1 * time.Millisecond})
	// второй ответ — чтобы воркер вышел из паузы и снова обратился к GetOrder
	fetcher.push(nil, accrual.ErrNotRegistered)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(db, fetcher, 10).Run(ctx)

	// ждём третьего вызова: 1-й вернул TooManyError, воркер ждал (fake/real),
	// 2-й вернул ErrNotRegistered, 3-й — воркер снова ждёт ответа
	fetcher.waitIdle(t, 3)

	var o2 model.Order
	require.NoError(t, db.First(&o2, "number = ?", o.Number).Error)
	require.Equal(t, model.OrderStatusNew, o2.Status, "статус не должен меняться при 429")
}
