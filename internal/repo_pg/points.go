package repopg

import (
	"context"

	"gorm.io/gorm"

	"github.com/gearwheels/gophermart_cp1/internal/model"
)

// OrderRepository предоставляет доступ к записям Order в базе данных.
// Встраивает универсальный Repository для стандартных CRUD-операций и
// добавляет предметно-специфичные методы запросов.
type OrderRepository struct {
	*Repository[model.Order]
	db *gorm.DB
}

// NewOrderRepository создаёт OrderRepository на основе db.
func NewOrderRepository(db *gorm.DB) *OrderRepository {
	return &OrderRepository{
		Repository: NewRepository[model.Order](db),
		db:         db,
	}
}

// GetByUserID возвращает все заказы пользователя userID, отсортированные
// от новых к старым по полю uploaded_at.
func (r *OrderRepository) GetByUserID(ctx context.Context, userID int64) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("uploaded_at DESC").
		Find(&orders).Error
	return orders, err
}

// GetProcessingBatch выбирает не более limit заказов в статусе NEW или
// PROCESSING, упорядоченных по uploaded_at по возрастанию, чтобы более
// старые заказы повторно обрабатывались первыми. Вызывается воркером
// начисления при каждой итерации опроса.
func (r *OrderRepository) GetProcessingBatch(ctx context.Context, limit int) ([]model.Order, error) {
	var orders []model.Order
	err := r.db.WithContext(ctx).
		Where("status IN ?", []model.OrderStatus{model.OrderStatusNew, model.OrderStatusProcessing}).
		Order("uploaded_at ASC").
		Limit(limit).
		Find(&orders).Error
	return orders, err
}

// WithdrawalRepository предоставляет доступ к записям Withdrawal в базе данных.
type WithdrawalRepository struct {
	*Repository[model.Withdrawal]
	db *gorm.DB
}

// NewWithdrawalRepository создаёт WithdrawalRepository на основе db.
func NewWithdrawalRepository(db *gorm.DB) *WithdrawalRepository {
	return &WithdrawalRepository{
		Repository: NewRepository[model.Withdrawal](db),
		db:         db,
	}
}

// GetByUserID возвращает все списания пользователя userID, отсортированные
// от новых к старым по полю processed_at.
func (r *WithdrawalRepository) GetByUserID(ctx context.Context, userID int64) ([]model.Withdrawal, error) {
	var w []model.Withdrawal
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("processed_at DESC").
		Find(&w).Error
	return w, err
}
