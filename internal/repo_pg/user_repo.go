package repopg

import (
	"context"

	"gorm.io/gorm"

	"github.com/gearwheels/gophermart_cp1/internal/model"
)

// UserRepository предоставляет доступ к записям User в базе данных.
// Встраивает универсальный Repository для стандартных CRUD-операций и
// добавляет методы поиска, специфичные для пользователей.
type UserRepository struct {
	*Repository[model.User]
	db *gorm.DB
}

// NewUserRepository создаёт UserRepository на основе db.
func NewUserRepository(db *gorm.DB) *UserRepository {
	return &UserRepository{
		Repository: NewRepository[model.User](db),
		db:         db,
	}
}

// GetByLogin возвращает пользователя с точным совпадением по полю login.
// Возвращает gorm.ErrRecordNotFound, если такой пользователь не найден.
func (r *UserRepository) GetByLogin(ctx context.Context, login string) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).Where("login = ?", login).First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
