// Пакет settings загружает и предоставляет конфигурацию приложения.
// Значения считываются сначала из переменных окружения; флаги командной строки
// (-a, -d, -r) имеют приоритет, если указаны. Разобранная конфигурация
// сохраняется в пакетной переменной AppConfig, доступной из любого места
// без внедрения зависимостей.
package settings

import (
	"flag"
	"io"
	"log/slog"
	"os"

	"github.com/caarlos0/env/v6"
)

// Config содержит всю конфигурацию времени выполнения сервиса gophermart.
type Config struct {
	// RunAddress — TCP-адрес, на котором слушает HTTP-сервер
	// (переменная окружения: RUN_ADDRESS, флаг: -a).
	RunAddress string `env:"RUN_ADDRESS" envDefault:"127.0.0.1:8080"`
	// AccrualSystemAddress — базовый URL внешней системы начисления баллов
	// (переменная окружения: ACCRUAL_SYSTEM_ADDRESS, флаг: -r).
	// Пустая строка отключает воркер начисления.
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS" envDefault:""`
	// DatabaseURI — строка подключения к PostgreSQL
	// (переменная окружения: DATABASE_URI, флаг: -d).
	DatabaseURI string `env:"DATABASE_URI" envDefault:"postgres://gophermart:gophermart@localhost:5432/gophermart"`
	// SecretKeyForJWT — ключ HMAC-SHA256 для подписи сессионных cookie
	// (переменная окружения: SECRET_KEY_FOR_JWT).
	SecretKeyForJWT string `env:"SECRET_KEY_FOR_JWT" envDefault:"MjIqdIvZJG2dXCd1E4Q/lJ//77eHRk6PrVM45hGhmcY="`
	// WorkerNum — максимальное количество заказов, выбираемых за одну итерацию
	// воркера начисления (переменная окружения: WORKER_NUM).
	WorkerNum int `env:"WORKER_NUM" envDefault:"10"`
}

// AppConfig — глобальный экземпляр конфигурации, заполняемый функциями Init
// или InitFromArgs. Должен быть установлен до того, как его начнут читать
// другие пакеты.
var AppConfig *Config

// Init загружает конфигурацию из переменных окружения и флагов os.Args[1:],
// сохраняет результат в AppConfig и возвращает его.
func Init() *Config {
	return InitFromArgs(os.Args[1:])
}

// InitFromArgs загружает конфигурацию из переменных окружения и переданного
// среза args (удобно для тестов с нестандартными флагами без изменения
// os.Args). Значения флагов переопределяют соответствующие переменные
// окружения.
func InitFromArgs(args []string) *Config {
	fs := flag.NewFlagSet("gophermart", flag.ContinueOnError)
	fs.SetOutput(io.Discard) // подавляем вывод в тестах
	a := fs.String("a", "", "адрес и порт запуска сервиса")              // localhost:8080
	d := fs.String("d", "", "строка подключения к базе данных")          // postgres://gophermart:gophermart@localhost:5432/gophermart
	r := fs.String("r", "", "адрес системы расчёта начислений баллов") //

	// При наличии неизвестных флагов (например, при `go test`) Parse возвращает
	// ошибку. Намеренно игнорируем её и опираемся на значения из env.
	_ = fs.Parse(args)

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		slog.Error("ошибка инициализации конфигурации", slog.String("err", err.Error()))
	}

	AppConfig = &Config{
		RunAddress:           cfg.RunAddress,
		AccrualSystemAddress: cfg.AccrualSystemAddress,
		DatabaseURI:          cfg.DatabaseURI,
		SecretKeyForJWT:      cfg.SecretKeyForJWT,
		WorkerNum:            cfg.WorkerNum,
	}

	if *a != "" {
		AppConfig.RunAddress = *a
	}
	if *d != "" {
		AppConfig.DatabaseURI = *d
	}
	if *r != "" {
		AppConfig.AccrualSystemAddress = *r
	}
	return AppConfig
}
