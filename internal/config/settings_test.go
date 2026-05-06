package settings

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInitFromArgs_FlagsOverrideEnv(t *testing.T) {
	t.Setenv("RUN_ADDRESS", "127.0.0.1:9999")
	t.Setenv("DATABASE_URI", "postgres://env")
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "http://env")

	cfg := InitFromArgs([]string{"-a", "0.0.0.0:8080", "-d", "postgres://flag", "-r", "http://flag"})
	require.Equal(t, "0.0.0.0:8080", cfg.RunAddress)
	require.Equal(t, "postgres://flag", cfg.DatabaseURI)
	require.Equal(t, "http://flag", cfg.AccrualSystemAddress)
}

func TestInitFromArgs_UnknownFlagsDontCrash(t *testing.T) {
	// simulate go test args
	_ = os.Setenv("RUN_ADDRESS", "127.0.0.1:7777")
	cfg := InitFromArgs([]string{"-test.v", "-unknown", "x"})
	require.Equal(t, "127.0.0.1:7777", cfg.RunAddress)
}

