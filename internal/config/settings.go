package settings

import (
	"flag"
	"log/slog"

	"github.com/caarlos0/env/v6"
)

type Config struct {
	RunAddress           string `env:"RUN_ADDRESS" envDefault:"127.0.0.1:8080"` // Адрес сервера
	AccrualSystemAddress string `env:"ACCRUAL_SYSTEM_ADDRESS" envDefault:""`
	DatabaseURI          string `env:"DATABASE_URI" envDefault:"postgres://gophermart:gophermart@localhost:5432/gophermart"`
	SecretKeyForJWT      string `env:"SECRET_KEY_FOR_JWT" envDefault:"MjIqdIvZJG2dXCd1E4Q/lJ//77eHRk6PrVM45hGhmcY="`
	WorkerNum            int    `env:"WORKER_NUM" envDefault:"10"`
}

var AppConfig *Config

func Init() *Config{
	a := flag.String("a", "", "start up address for the server")           // localhost:8080
	d := flag.String("d", "", "database URI")                              // postgres://gophermart:gophermart@localhost:5432/gophermart
	r := flag.String("r", "", "address of the accrual calculation system") //
	flag.Parse()

	cfg := &Config{}
	if err := env.Parse(cfg); err != nil {
		slog.Error("ERROR in config initialization", err)
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
