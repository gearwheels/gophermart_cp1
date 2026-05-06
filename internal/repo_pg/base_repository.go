// Пакет repopg предоставляет реализации репозиториев для работы с PostgreSQL
// через GORM. Каждый репозиторий встраивает универсальный Repository[T]
// для стандартных операций CRUD и добавляет поверх него предметно-специфичные
// методы запросов.
package repopg

import (
	"context"

	"gorm.io/gorm"
)

// Repository — универсальный CRUD-репозиторий на основе соединения GORM.
// T должен быть структурой модели GORM. Встраивайте Repository[T] в
// предметно-специфичные типы репозиториев, чтобы наследовать стандартные
// операции CRUD без повторения кода.
type Repository[T any] struct {
	db *gorm.DB
}

// NewRepository создаёт новый экземпляр репозитория.
func NewRepository[T any](db *gorm.DB) *Repository[T] {
	return &Repository[T]{db: db}
}

// Create – вставка записи. Возвращает созданную модель (с заполненным ID).
func (r *Repository[T]) Create(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Create(entity).Error
}

// GetByID – получение записи по первичному ключу.
func (r *Repository[T]) GetByID(ctx context.Context, id uint) (*T, error) {
	var entity T
	err := r.db.WithContext(ctx).First(&entity, id).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// Update – обновление всей записи (сохраняет все поля).
// Для выборочного обновления используйте UpdateFields.
func (r *Repository[T]) Update(ctx context.Context, entity *T) error {
	return r.db.WithContext(ctx).Save(entity).Error
}

// UpdateFields – обновление только указанных полей по ID.
func (r *Repository[T]) UpdateFields(ctx context.Context, id uint, fields map[string]interface{}) error {
	return r.db.WithContext(ctx).Model(new(T)).Where("id = ?", id).Updates(fields).Error
}

// Delete – мягкое удаление (если в модели есть gorm.DeletedAt) или жёсткое.
func (r *Repository[T]) Delete(ctx context.Context, id uint) error {
	return r.db.WithContext(ctx).Delete(new(T), id).Error
}

// GetAll – получение всех записей.
func (r *Repository[T]) GetAll(ctx context.Context) ([]T, error) {
	var entities []T
	err := r.db.WithContext(ctx).Find(&entities).Error
	return entities, err
}

// FirstWhere – получение первой записи по условию (map или struct).
func (r *Repository[T]) FirstWhere(ctx context.Context, condition interface{}, args ...interface{}) (*T, error) {
	var entity T
	err := r.db.WithContext(ctx).Where(condition, args...).First(&entity).Error
	if err != nil {
		return nil, err
	}
	return &entity, nil
}

// FindWhere – получение всех записей по условию.
func (r *Repository[T]) FindWhere(ctx context.Context, dest interface{}, condition interface{}, args ...interface{}) error {
	return r.db.WithContext(ctx).Where(condition, args...).Find(dest).Error
}
