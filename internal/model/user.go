// Пакет model содержит основные доменные типы, соответствующие таблицам базы
// данных. Все типы — простые структуры Go с тегами GORM; бизнес-логика здесь
// отсутствует.
package model

import (
	"time"

	"gorm.io/gorm"
)

// User представляет зарегистрированного пользователя системы Гофермарт.
// Таблица: "users".
type User struct {
	// ID — автоинкрементный первичный ключ.
	ID int64 `gorm:"primaryKey"`
	// Login — уникальный логин, выбранный при регистрации.
	Login string `gorm:"column:login;uniqueIndex;not null"`
	// PasswordHash — хэш пароля, вычисленный bcrypt; исходный текст не хранится.
	PasswordHash string `gorm:"column:password_hash;not null"`
	// CurrentBalance — текущий остаток баллов лояльности в виртуальных единицах.
	CurrentBalance float64 `gorm:"column:current_balance;not null;default:0"`
	// Withdrawn — совокупная сумма баллов, потраченных с момента регистрации.
	Withdrawn float64   `gorm:"column:withdrawn;not null;default:0"`
	CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
	UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

// TableName переопределяет имя таблицы GORM по умолчанию.
func (User) TableName() string {
	return "users"
}

// OrderStatus — перечисление состояний обработки заказа.
type OrderStatus string

const (
	// OrderStatusNew означает, что заказ загружен, но ещё не передан в систему
	// начисления.
	OrderStatusNew OrderStatus = "NEW"
	// OrderStatusProcessing означает, что система начисления рассчитывает
	// вознаграждение за этот заказ.
	OrderStatusProcessing OrderStatus = "PROCESSING"
	// OrderStatusInvalid означает, что система начисления отказала в расчёте;
	// вознаграждение начислено не будет. Это терминальный статус.
	OrderStatusInvalid OrderStatus = "INVALID"
	// OrderStatusProcessed означает, что расчёт начисления завершён и
	// вознаграждение зачислено на баланс пользователя. Это терминальный статус.
	OrderStatusProcessed OrderStatus = "PROCESSED"
)

// Order фиксирует заказ лояльности, загруженный пользователем.
// Таблица: "orders".
type Order struct {
	// Number — уникальный идентификатор заказа (первичный ключ); проверяется
	// по алгоритму Луна перед сохранением.
	Number string `gorm:"primaryKey;column:number"`
	// UserID — внешний ключ, ссылающийся на владельца заказа.
	UserID int64 `gorm:"column:user_id;not null;index"`
	// Status — текущее состояние обработки заказа.
	Status OrderStatus `gorm:"column:status;type:varchar(16);not null"`
	// Accrual — сумма вознаграждения в баллах, возвращённая системой начисления.
	// Равна nil до получения результата от системы начисления.
	Accrual *float64 `gorm:"column:accrual"`
	// AccrualApplied равно true, когда вознаграждение уже добавлено на баланс
	// пользователя, что исключает повторное начисление при повторных попытках.
	AccrualApplied bool      `gorm:"column:accrual_applied;not null;default:false"`
	UploadedAt     time.Time `gorm:"column:uploaded_at;autoCreateTime"`
}

// TableName переопределяет имя таблицы GORM по умолчанию.
func (Order) TableName() string { return "orders" }

// Withdrawal фиксирует одну операцию списания баллов лояльности пользователем.
// Таблица: "withdrawals".
type Withdrawal struct {
	// ID — автоинкрементный первичный ключ.
	ID int64 `gorm:"primaryKey"`
	// UserID — внешний ключ, ссылающийся на пользователя, выполнившего списание.
	UserID int64 `gorm:"column:user_id;not null;index"`
	// OrderNumber — номер заказа, в счёт оплаты которого списываются баллы.
	OrderNumber string `gorm:"column:order_number;not null;index"`
	// Sum — количество баллов лояльности, списанных с баланса пользователя.
	Sum         float64   `gorm:"column:sum;not null"`
	ProcessedAt time.Time `gorm:"column:processed_at;not null"`
}

// TableName переопределяет имя таблицы GORM по умолчанию.
func (Withdrawal) TableName() string { return "withdrawals" }

// Предотвращает ошибку «неиспользуемый импорт» для gorm в этом пакете.
var _ = gorm.ErrRecordNotFound
