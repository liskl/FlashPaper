// Package config handles loading and parsing of FlashPaper configuration.
// Configuration can come from an INI file (PrivateBin-compatible format)
// and/or environment variables. Environment variables take precedence,
// following the 12-factor app methodology.
//
// The configuration is organized into sections matching PrivateBin:
//   - [main]: Core application settings (name, template, size limits)
//   - [expire]: Paste expiration options and defaults
//   - [traffic]: Rate limiting configuration
//   - [purge]: Expired paste cleanup settings
//   - [model]: Storage backend configuration
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"gopkg.in/ini.v1"
)

// Config holds all application configuration organized by section.
// This structure mirrors PrivateBin's conf.php for API compatibility.
type Config struct {
	Main    MainConfig
	Expire  ExpireConfig
	Traffic TrafficConfig
	Purge   PurgeConfig
	Model   ModelConfig
}

// MainConfig contains core application settings.
type MainConfig struct {
	// Name is the application title shown in the browser
	Name string

	// Host is the address to bind the HTTP server to (default: 0.0.0.0)
	Host string

	// Port is the HTTP server port (default: 8080)
	Port int

	// BasePath is the URL path prefix (useful when behind a reverse proxy)
	BasePath string

	// Discussion enables or disables the comment/discussion feature
	Discussion bool

	// OpenDiscussion allows discussions without requiring a paste password
	OpenDiscussion bool

	// Password enables password protection option for pastes
	Password bool

	// FileUpload enables file attachment support
	FileUpload bool

	// BurnAfterReadingSelected sets burn-after-reading as the default option
	BurnAfterReadingSelected bool

	// SizeLimit is the maximum paste size in bytes (default: 10MB)
	SizeLimit int64

	// Template is the UI template to use (bootstrap5, bootstrap-dark, etc.)
	Template string

	// LanguageSelection enables the language picker in the UI
	LanguageSelection bool

	// LanguageDefault is the default language code (e.g., "en")
	LanguageDefault string

	// QRCode enables QR code generation for paste URLs
	QRCode bool

	// Icon sets the icon style for comments (identicon, jdenticon, vizhash, none)
	Icon string

	// HTTPWarning shows a warning when not using HTTPS
	HTTPWarning bool

	// Compression specifies the compression algorithm (zlib or none)
	Compression string
}

// ExpireConfig controls paste expiration behavior.
type ExpireConfig struct {
	// Default is the default expiration option (e.g., "1week")
	Default string

	// Options maps expiration labels to durations
	// Standard options: 5min, 10min, 1hour, 1day, 1week, 1month, 1year, never
	Options map[string]time.Duration
}

// TrafficConfig controls rate limiting to prevent abuse.
type TrafficConfig struct {
	// Limit is the minimum seconds between paste creations from same IP
	// Set to 0 to disable rate limiting
	Limit int

	// Exempted is a list of IP addresses/subnets exempt from rate limiting
	Exempted []string

	// Creators is a list of IPs that are allowed to create pastes
	// If empty, all IPs can create pastes
	Creators []string

	// Header is the HTTP header to use for client IP (for reverse proxies)
	// Common values: X-Forwarded-For, X-Real-IP, CF-Connecting-IP
	Header string
}

// PurgeConfig controls automatic cleanup of expired pastes.
type PurgeConfig struct {
	// Limit is the minimum seconds between purge operations
	// Set to 0 to disable automatic purging
	Limit int

	// BatchSize is the number of pastes to delete per purge cycle
	BatchSize int
}

// ModelConfig defines the storage backend settings.
type ModelConfig struct {
	// Class is the storage backend type: Database or Filesystem
	Class string

	// Database-specific settings (when Class = "Database")
	DSN    string // Data Source Name for database connection
	Driver string // Database driver: sqlite3, postgres, mysql

	// Filesystem-specific settings (when Class = "Filesystem")
	Dir string // Directory path for paste storage
}

// DefaultConfig returns a Config with sensible defaults matching PrivateBin.
// These defaults provide a secure, functional starting point.
func DefaultConfig() *Config {
	return &Config{
		Main: MainConfig{
			Name:                     "FlashPaper",
			Host:                     "0.0.0.0",
			Port:                     8080,
			BasePath:                 "",
			Discussion:               true,
			OpenDiscussion:           false,
			Password:                 true,
			FileUpload:               false,
			BurnAfterReadingSelected: false,
			SizeLimit:                10 * 1024 * 1024, // 10 MiB
			Template:                 "bootstrap5",
			LanguageSelection:        false,
			LanguageDefault:          "en",
			QRCode:                   true,
			Icon:                     "identicon",
			HTTPWarning:              true,
			Compression:              "zlib",
		},
		Expire: ExpireConfig{
			Default: "1week",
			Options: map[string]time.Duration{
				"5min":   5 * time.Minute,
				"10min":  10 * time.Minute,
				"1hour":  1 * time.Hour,
				"1day":   24 * time.Hour,
				"1week":  7 * 24 * time.Hour,
				"1month": 30 * 24 * time.Hour,
				"1year":  365 * 24 * time.Hour,
				"never":  0, // 0 means no expiration
			},
		},
		Traffic: TrafficConfig{
			Limit:     10, // 10 seconds between pastes
			Exempted:  []string{},
			Creators:  []string{},
			Header:    "",
		},
		Purge: PurgeConfig{
			Limit:     300, // 5 minutes between purge runs
			BatchSize: 10,
		},
		Model: ModelConfig{
			Class:  "Database",
			Driver: "sqlite3",
			DSN:    "flashpaper.db",
			Dir:    "data",
		},
	}
}

// Load reads configuration from an INI file and environment variables.
// Environment variables override file settings. If the config file doesn't
// exist, default values are used.
//
// Environment variable format: FLASHPAPER_SECTION_KEY
// Example: FLASHPAPER_MAIN_PORT=9090
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	// Try to load from INI file if it exists
	if _, err := os.Stat(path); err == nil {
		if err := cfg.loadFromFile(path); err != nil {
			return nil, fmt.Errorf("parsing config file: %w", err)
		}
	}

	// Override with environment variables
	cfg.loadFromEnv()

	// Validate the final configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return cfg, nil
}

// loadFromFile parses an INI configuration file.
func (c *Config) loadFromFile(path string) error {
	iniFile, err := ini.Load(path)
	if err != nil {
		return err
	}

	// [main] section
	if sec, err := iniFile.GetSection("main"); err == nil {
		c.Main.Name = sec.Key("name").MustString(c.Main.Name)
		c.Main.Host = sec.Key("host").MustString(c.Main.Host)
		c.Main.Port = sec.Key("port").MustInt(c.Main.Port)
		c.Main.BasePath = sec.Key("basepath").MustString(c.Main.BasePath)
		c.Main.Discussion = sec.Key("discussion").MustBool(c.Main.Discussion)
		c.Main.OpenDiscussion = sec.Key("opendiscussion").MustBool(c.Main.OpenDiscussion)
		c.Main.Password = sec.Key("password").MustBool(c.Main.Password)
		c.Main.FileUpload = sec.Key("fileupload").MustBool(c.Main.FileUpload)
		c.Main.BurnAfterReadingSelected = sec.Key("burnafterreadingselected").MustBool(c.Main.BurnAfterReadingSelected)
		c.Main.SizeLimit = sec.Key("sizelimit").MustInt64(c.Main.SizeLimit)
		c.Main.Template = sec.Key("template").MustString(c.Main.Template)
		c.Main.LanguageSelection = sec.Key("languageselection").MustBool(c.Main.LanguageSelection)
		c.Main.LanguageDefault = sec.Key("languagedefault").MustString(c.Main.LanguageDefault)
		c.Main.QRCode = sec.Key("qrcode").MustBool(c.Main.QRCode)
		c.Main.Icon = sec.Key("icon").MustString(c.Main.Icon)
		c.Main.HTTPWarning = sec.Key("httpwarning").MustBool(c.Main.HTTPWarning)
		c.Main.Compression = sec.Key("compression").MustString(c.Main.Compression)
	}

	// [expire] section
	if sec, err := iniFile.GetSection("expire"); err == nil {
		c.Expire.Default = sec.Key("default").MustString(c.Expire.Default)
	}

	// [expire_options] section - custom expiration times
	if sec, err := iniFile.GetSection("expire_options"); err == nil {
		for _, key := range sec.Keys() {
			seconds := key.MustInt64(0)
			if seconds == 0 {
				c.Expire.Options[key.Name()] = 0 // "never" expiration
			} else {
				c.Expire.Options[key.Name()] = time.Duration(seconds) * time.Second
			}
		}
	}

	// [traffic] section
	if sec, err := iniFile.GetSection("traffic"); err == nil {
		c.Traffic.Limit = sec.Key("limit").MustInt(c.Traffic.Limit)
		c.Traffic.Header = sec.Key("header").MustString(c.Traffic.Header)

		if exempted := sec.Key("exempted").MustString(""); exempted != "" {
			c.Traffic.Exempted = strings.Split(exempted, ",")
			for i := range c.Traffic.Exempted {
				c.Traffic.Exempted[i] = strings.TrimSpace(c.Traffic.Exempted[i])
			}
		}

		if creators := sec.Key("creators").MustString(""); creators != "" {
			c.Traffic.Creators = strings.Split(creators, ",")
			for i := range c.Traffic.Creators {
				c.Traffic.Creators[i] = strings.TrimSpace(c.Traffic.Creators[i])
			}
		}
	}

	// [purge] section
	if sec, err := iniFile.GetSection("purge"); err == nil {
		c.Purge.Limit = sec.Key("limit").MustInt(c.Purge.Limit)
		c.Purge.BatchSize = sec.Key("batchsize").MustInt(c.Purge.BatchSize)
	}

	// [model] section
	if sec, err := iniFile.GetSection("model"); err == nil {
		c.Model.Class = sec.Key("class").MustString(c.Model.Class)
		c.Model.Driver = sec.Key("driver").MustString(c.Model.Driver)
		c.Model.DSN = sec.Key("dsn").MustString(c.Model.DSN)
		c.Model.Dir = sec.Key("dir").MustString(c.Model.Dir)
	}

	return nil
}

// loadFromEnv overrides configuration with environment variables.
// Format: FLASHPAPER_SECTION_KEY (e.g., FLASHPAPER_MAIN_PORT)
func (c *Config) loadFromEnv() {
	// Main section
	if v := os.Getenv("FLASHPAPER_MAIN_NAME"); v != "" {
		c.Main.Name = v
	}
	if v := os.Getenv("FLASHPAPER_MAIN_HOST"); v != "" {
		c.Main.Host = v
	}
	if v := os.Getenv("FLASHPAPER_MAIN_PORT"); v != "" {
		if port, err := strconv.Atoi(v); err == nil {
			c.Main.Port = port
		}
	}
	if v := os.Getenv("FLASHPAPER_MAIN_BASEPATH"); v != "" {
		c.Main.BasePath = v
	}
	if v := os.Getenv("FLASHPAPER_MAIN_SIZELIMIT"); v != "" {
		if size, err := strconv.ParseInt(v, 10, 64); err == nil {
			c.Main.SizeLimit = size
		}
	}

	// Model section (storage backend)
	if v := os.Getenv("FLASHPAPER_MODEL_CLASS"); v != "" {
		c.Model.Class = v
	}
	if v := os.Getenv("FLASHPAPER_MODEL_DRIVER"); v != "" {
		c.Model.Driver = v
	}
	if v := os.Getenv("FLASHPAPER_MODEL_DSN"); v != "" {
		c.Model.DSN = v
	}
	if v := os.Getenv("FLASHPAPER_MODEL_DIR"); v != "" {
		c.Model.Dir = v
	}

	// Shorthand environment variables for Docker compatibility
	if v := os.Getenv("FLASHPAPER_DB_TYPE"); v != "" {
		c.Model.Class = "Database"
		c.Model.Driver = v
	}
	if v := os.Getenv("FLASHPAPER_DB_HOST"); v != "" {
		// Construct DSN based on driver type
		c.updateDSNFromEnv()
	}

	// Traffic section
	if v := os.Getenv("FLASHPAPER_TRAFFIC_LIMIT"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			c.Traffic.Limit = limit
		}
	}

	// Purge section
	if v := os.Getenv("FLASHPAPER_PURGE_LIMIT"); v != "" {
		if limit, err := strconv.Atoi(v); err == nil {
			c.Purge.Limit = limit
		}
	}
}

// updateDSNFromEnv constructs a database DSN from individual environment variables.
// This provides a more Docker-friendly configuration approach.
func (c *Config) updateDSNFromEnv() {
	host := os.Getenv("FLASHPAPER_DB_HOST")
	port := os.Getenv("FLASHPAPER_DB_PORT")
	user := os.Getenv("FLASHPAPER_DB_USER")
	pass := os.Getenv("FLASHPAPER_DB_PASSWORD")
	name := os.Getenv("FLASHPAPER_DB_NAME")

	// Set defaults based on driver
	if port == "" {
		switch c.Model.Driver {
		case "postgres":
			port = "5432"
		case "mysql":
			port = "3306"
		}
	}
	if name == "" {
		name = "flashpaper"
	}

	// Construct driver-specific DSN
	switch c.Model.Driver {
	case "postgres":
		// PostgreSQL DSN format: postgres://user:pass@host:port/dbname?sslmode=disable
		c.Model.DSN = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=disable",
			user, pass, host, port, name)
	case "mysql":
		// MySQL DSN format: user:pass@tcp(host:port)/dbname?parseTime=true
		c.Model.DSN = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?parseTime=true",
			user, pass, host, port, name)
	}
}

// Validate checks that the configuration is valid and consistent.
func (c *Config) Validate() error {
	// Port must be in valid range
	if c.Main.Port < 1 || c.Main.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Main.Port)
	}

	// Size limit must be positive
	if c.Main.SizeLimit <= 0 {
		return fmt.Errorf("sizelimit must be positive, got %d", c.Main.SizeLimit)
	}

	// Default expiration must be a valid option
	if _, ok := c.Expire.Options[c.Expire.Default]; !ok {
		return fmt.Errorf("default expiration %q is not a valid option", c.Expire.Default)
	}

	// Storage class must be valid
	switch c.Model.Class {
	case "Database", "Filesystem":
		// Valid
	default:
		return fmt.Errorf("model class must be 'Database' or 'Filesystem', got %q", c.Model.Class)
	}

	// Database driver must be valid when using Database class
	if c.Model.Class == "Database" {
		switch c.Model.Driver {
		case "sqlite3", "postgres", "mysql":
			// Valid
		default:
			return fmt.Errorf("database driver must be 'sqlite3', 'postgres', or 'mysql', got %q", c.Model.Driver)
		}
	}

	// Icon type must be valid
	switch c.Main.Icon {
	case "identicon", "jdenticon", "vizhash", "none":
		// Valid
	default:
		return fmt.Errorf("icon must be 'identicon', 'jdenticon', 'vizhash', or 'none', got %q", c.Main.Icon)
	}

	// Compression must be valid
	switch c.Main.Compression {
	case "zlib", "none":
		// Valid
	default:
		return fmt.Errorf("compression must be 'zlib' or 'none', got %q", c.Main.Compression)
	}

	return nil
}

// GetExpireDuration returns the duration for a given expiration option.
// Returns 0 (meaning never expires) if the option is not found.
func (c *Config) GetExpireDuration(option string) time.Duration {
	if d, ok := c.Expire.Options[option]; ok {
		return d
	}
	return 0
}
