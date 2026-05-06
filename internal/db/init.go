// Пакет db отвечает за установку соединения с PostgreSQL и запуск миграций
// базы данных. InitDB — единственная точка входа: открывает GORM-соединение
// и выполняет все ожидающие Goose-миграции из каталога ./migrations, после
// чего возвращает готовый к использованию экземпляр *gorm.DB.
package db

import (
	"database/sql"
	"path/filepath"

	"github.com/pressly/goose/v3"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

// InitDB открывает PostgreSQL-соединение по строке connStr, выполняет все
// ожидающие Goose-миграции из каталога ./migrations и возвращает
// инициализированный *gorm.DB. Возвращает ошибку, если соединение не удалось
// установить или хотя бы одна миграция завершилась неудачей.
func InitDB(connStr string) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(connStr), &gorm.Config{})
	if err != nil {
		return nil, err
	}

	sqlDB, err := gdb.DB()
	if err != nil {
		return nil, err
	}

	if err := runMigrations(sqlDB, filepath.Join(".", "migrations"), "postgres"); err != nil {
		return nil, err
	}

	return gdb, nil
}

func runMigrations(sqlDB *sql.DB, dir string, dialect string) error {
	if err := goose.SetDialect(dialect); err != nil {
		return err
	}
	return goose.Up(sqlDB, dir)
}
