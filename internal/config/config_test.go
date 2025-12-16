package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig_HasSensibleDefaults(t *testing.T) {
	cfg := DefaultConfig()

	// Main section defaults
	assert.Equal(t, "FlashPaper", cfg.Main.Name)
	assert.Equal(t, "0.0.0.0", cfg.Main.Host)
	assert.Equal(t, 8080, cfg.Main.Port)
	assert.True(t, cfg.Main.Discussion)
	assert.True(t, cfg.Main.Password)
	assert.False(t, cfg.Main.FileUpload)
	assert.Equal(t, int64(10*1024*1024), cfg.Main.SizeLimit) // 10 MiB

	// Expire section defaults
	assert.Equal(t, "1week", cfg.Expire.Default)
	assert.Len(t, cfg.Expire.Options, 8) // 5min, 10min, 1hour, 1day, 1week, 1month, 1year, never

	// Traffic section defaults
	assert.Equal(t, 10, cfg.Traffic.Limit)

	// Purge section defaults
	assert.Equal(t, 300, cfg.Purge.Limit)
	assert.Equal(t, 10, cfg.Purge.BatchSize)

	// Model section defaults
	assert.Equal(t, "Database", cfg.Model.Class)
	assert.Equal(t, "sqlite3", cfg.Model.Driver)
}

func TestConfig_Validate_ValidConfig(t *testing.T) {
	cfg := DefaultConfig()
	err := cfg.Validate()
	assert.NoError(t, err)
}

func TestConfig_Validate_InvalidPort(t *testing.T) {
	tests := []struct {
		name string
		port int
	}{
		{"zero port", 0},
		{"negative port", -1},
		{"too high port", 65536},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Main.Port = tt.port
			err := cfg.Validate()
			assert.Error(t, err)
			assert.Contains(t, err.Error(), "port")
		})
	}
}

func TestConfig_Validate_InvalidSizeLimit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Main.SizeLimit = 0
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "sizelimit")
}

func TestConfig_Validate_InvalidExpireDefault(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Expire.Default = "invalid"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "default expiration")
}

func TestConfig_Validate_InvalidModelClass(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model.Class = "InvalidClass"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "model class")
}

func TestConfig_Validate_InvalidDriver(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Model.Class = "Database"
	cfg.Model.Driver = "invalid"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "database driver")
}

func TestConfig_Validate_InvalidIcon(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Main.Icon = "invalid"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "icon")
}

func TestConfig_Validate_InvalidCompression(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Main.Compression = "invalid"
	err := cfg.Validate()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "compression")
}

func TestLoad_NonExistentFile_UsesDefaults(t *testing.T) {
	cfg, err := Load("/nonexistent/path/config.ini")
	require.NoError(t, err)
	assert.Equal(t, DefaultConfig().Main.Port, cfg.Main.Port)
}

func TestLoad_FromFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")

	content := `
[main]
name = TestPaper
port = 9090
discussion = false
sizelimit = 5242880

[expire]
default = 1day

[traffic]
limit = 30

[purge]
limit = 600
batchsize = 20

[model]
class = Database
driver = postgres
dsn = postgres://user:pass@localhost/testdb
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Equal(t, "TestPaper", cfg.Main.Name)
	assert.Equal(t, 9090, cfg.Main.Port)
	assert.False(t, cfg.Main.Discussion)
	assert.Equal(t, int64(5242880), cfg.Main.SizeLimit)
	assert.Equal(t, "1day", cfg.Expire.Default)
	assert.Equal(t, 30, cfg.Traffic.Limit)
	assert.Equal(t, 600, cfg.Purge.Limit)
	assert.Equal(t, 20, cfg.Purge.BatchSize)
	assert.Equal(t, "Database", cfg.Model.Class)
	assert.Equal(t, "postgres", cfg.Model.Driver)
	assert.Equal(t, "postgres://user:pass@localhost/testdb", cfg.Model.DSN)
}

func TestLoad_EnvOverridesFile(t *testing.T) {
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")

	content := `
[main]
port = 8080
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	// Set environment variable to override
	os.Setenv("FLASHPAPER_MAIN_PORT", "9999")
	defer os.Unsetenv("FLASHPAPER_MAIN_PORT")

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Environment variable should win
	assert.Equal(t, 9999, cfg.Main.Port)
}

func TestLoad_EnvDatabaseConfig(t *testing.T) {
	// Test Docker-style environment variables
	os.Setenv("FLASHPAPER_DB_TYPE", "postgres")
	os.Setenv("FLASHPAPER_DB_HOST", "db.example.com")
	os.Setenv("FLASHPAPER_DB_PORT", "5432")
	os.Setenv("FLASHPAPER_DB_USER", "flashuser")
	os.Setenv("FLASHPAPER_DB_PASSWORD", "secret")
	os.Setenv("FLASHPAPER_DB_NAME", "flashdb")
	defer func() {
		os.Unsetenv("FLASHPAPER_DB_TYPE")
		os.Unsetenv("FLASHPAPER_DB_HOST")
		os.Unsetenv("FLASHPAPER_DB_PORT")
		os.Unsetenv("FLASHPAPER_DB_USER")
		os.Unsetenv("FLASHPAPER_DB_PASSWORD")
		os.Unsetenv("FLASHPAPER_DB_NAME")
	}()

	cfg, err := Load("/nonexistent/config.ini")
	require.NoError(t, err)

	assert.Equal(t, "Database", cfg.Model.Class)
	assert.Equal(t, "postgres", cfg.Model.Driver)
	assert.Contains(t, cfg.Model.DSN, "db.example.com")
	assert.Contains(t, cfg.Model.DSN, "flashuser")
	assert.Contains(t, cfg.Model.DSN, "flashdb")
}

func TestGetExpireDuration_ValidOptions(t *testing.T) {
	cfg := DefaultConfig()

	tests := []struct {
		option   string
		expected time.Duration
	}{
		{"5min", 5 * time.Minute},
		{"10min", 10 * time.Minute},
		{"1hour", 1 * time.Hour},
		{"1day", 24 * time.Hour},
		{"1week", 7 * 24 * time.Hour},
		{"1month", 30 * 24 * time.Hour},
		{"1year", 365 * 24 * time.Hour},
		{"never", 0},
	}

	for _, tt := range tests {
		t.Run(tt.option, func(t *testing.T) {
			d := cfg.GetExpireDuration(tt.option)
			assert.Equal(t, tt.expected, d)
		})
	}
}

func TestGetExpireDuration_InvalidOption(t *testing.T) {
	cfg := DefaultConfig()
	d := cfg.GetExpireDuration("invalid")
	assert.Equal(t, time.Duration(0), d)
}

func TestLoad_CustomExpireOptions(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")

	content := `
[expire]
default = 2hours

[expire_options]
2hours = 7200
12hours = 43200
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	// Should have both default and custom options
	assert.Equal(t, 2*time.Hour, cfg.GetExpireDuration("2hours"))
	assert.Equal(t, 12*time.Hour, cfg.GetExpireDuration("12hours"))
	// Default options should still exist
	assert.Equal(t, 5*time.Minute, cfg.GetExpireDuration("5min"))
}

func TestLoad_TrafficExemptedAndCreators(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.ini")

	content := `
[traffic]
limit = 10
exempted = 192.168.1.0/24, 10.0.0.1
creators = 192.168.1.100, 192.168.1.101
`
	err := os.WriteFile(configPath, []byte(content), 0644)
	require.NoError(t, err)

	cfg, err := Load(configPath)
	require.NoError(t, err)

	assert.Len(t, cfg.Traffic.Exempted, 2)
	assert.Contains(t, cfg.Traffic.Exempted, "192.168.1.0/24")
	assert.Contains(t, cfg.Traffic.Exempted, "10.0.0.1")

	assert.Len(t, cfg.Traffic.Creators, 2)
	assert.Contains(t, cfg.Traffic.Creators, "192.168.1.100")
	assert.Contains(t, cfg.Traffic.Creators, "192.168.1.101")
}

func TestConfig_ValidDrivers(t *testing.T) {
	drivers := []string{"sqlite3", "postgres", "mysql"}

	for _, driver := range drivers {
		t.Run(driver, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Model.Driver = driver
			err := cfg.Validate()
			assert.NoError(t, err)
		})
	}
}

func TestConfig_ValidStorageClasses(t *testing.T) {
	classes := []string{"Database", "Filesystem"}

	for _, class := range classes {
		t.Run(class, func(t *testing.T) {
			cfg := DefaultConfig()
			cfg.Model.Class = class
			err := cfg.Validate()
			assert.NoError(t, err)
		})
	}
}
