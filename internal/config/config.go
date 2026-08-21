package config

import (
	"fmt"

	"github.com/LyricTian/gin-admin/v10/pkg/encoding/json"
	"github.com/LyricTian/gin-admin/v10/pkg/logging"
)

type Config struct {
	Logger     logging.LoggerConfig
	General    General
	Storage    Storage
	Middleware Middleware
	Util       Util
	Dictionary Dictionary
	Trading    Trading
}

type Trading struct {
	AccountSync struct {
		Enabled               bool   `default:"true"`
		IntervalSeconds       int    `default:"300"`
		TimeoutSeconds        int    `default:"60"`
		SnapshotIntervalHours int    `default:"12"`
		SnapshotTimeZone      string `default:"Asia/Shanghai"`
	}
	UserStream struct {
		// Enabled starts read-only private WebSocket subscriptions. Reconciliation
		// remains REST-based and the stream never exposes trading methods.
		Enabled bool `default:"false"`
	}
	TelegramBot struct {
		// Enabled starts local-friendly Telegram long polling. It remains off by
		// default because polling and replies are external network side effects.
		Enabled            bool `default:"false"`
		PollTimeoutSeconds int  `default:"25"`
		RefreshSeconds     int  `default:"30"`
		BindingTTLMinutes  int  `default:"10"`
	}
	Notifications struct {
		// Enabled permits strategy decisions to be delivered automatically. Manual
		// channel tests remain available while this switch is disabled.
		Enabled         bool   `default:"false"`
		ApprovalBaseURL string `default:"http://127.0.0.1:8000/trading-data/approvals"`
	}
	MarketStream struct {
		// Enabled persists public Binance liquidation snapshots for the heatmap.
		Enabled        bool   `default:"false"`
		RetentionHours int    `default:"168"`
		Endpoint       string `default:"wss://fstream.binance.com/ws/!forceOrder@arr"`
	}
	Execution struct {
		// Enabled is the process-wide hard gate for any exchange trading request.
		// It is intentionally false unless explicitly configured by an operator.
		Enabled bool `default:"false"`
		// TestnetOnly prevents production providers from being constructed even
		// after Enabled is turned on. Keep true until production acceptance.
		TestnetOnly    bool `default:"true"`
		TimeoutSeconds int  `default:"15"`
	}
}

type General struct {
	AppName            string `default:"ginadmin"`
	Version            string `default:"v10.1.0"`
	Debug              bool
	PprofAddr          string
	DisableSwagger     bool
	DisablePrintConfig bool
	WorkDir            string // From command arguments
	MenuFile           string // From schema.Menus (JSON/YAML)
	DenyOperateMenu    bool
	Security           struct {
		// Strict enables production startup checks for authentication keys and credentials.
		Strict bool
	}
	HTTP struct {
		Addr            string `default:":8040"`
		ShutdownTimeout int    `default:"10"` // seconds
		ReadTimeout     int    `default:"60"` // seconds
		WriteTimeout    int    `default:"60"` // seconds
		IdleTimeout     int    `default:"10"` // seconds
		CertFile        string
		KeyFile         string
	}
	Root struct {
		ID           string `default:"root"`
		Username     string `default:"admin"`
		PasswordHash string
		Name         string `default:"Admin"`
	}
}

type Storage struct {
	Cache struct {
		Type      string `default:"memory"` // memory/badger/redis
		Delimiter string `default:":"`      // delimiter for key
		Memory    struct {
			CleanupInterval int `default:"60"` // seconds
		}
		Badger struct {
			Path string `default:"data/cache"`
		}
		Redis struct {
			Addr     string
			Username string
			Password string
			DB       int
		}
	}
	DB struct {
		Debug        bool
		Type         string `default:"sqlite3"`          // sqlite3/mysql/postgres
		DSN          string `default:"data/ginadmin.db"` // database source name
		MaxLifetime  int    `default:"86400"`            // seconds
		MaxIdleTime  int    `default:"3600"`             // seconds
		MaxOpenConns int    `default:"100"`              // connections
		MaxIdleConns int    `default:"50"`               // connections
		TablePrefix  string `default:""`
		AutoMigrate  bool
		PrepareStmt  bool
		Resolver     []struct {
			DBType   string   // sqlite3/mysql/postgres
			Sources  []string // DSN
			Replicas []string // DSN
			Tables   []string
		}
	}
}

type Util struct {
	Captcha struct {
		Length    int    `default:"4"`
		Width     int    `default:"400"`
		Height    int    `default:"160"`
		CacheType string `default:"memory"` // memory/redis
		Redis     struct {
			Addr      string
			Username  string
			Password  string
			DB        int
			KeyPrefix string `default:"captcha:"`
		}
	}
	Prometheus struct {
		Enable         bool
		Port           int    `default:"9100"`
		BasicUsername  string `default:"admin"`
		BasicPassword  string `default:"admin"`
		LogApis        []string
		LogMethods     []string
		DefaultCollect bool
	}
}

type Dictionary struct {
	UserCacheExp int `default:"4"` // hours
}

func (c *Config) IsDebug() bool {
	return c.General.Debug
}

func (c *Config) String() string {
	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		panic("Failed to marshal config: " + err.Error())
	}
	return string(b)
}

func (c *Config) PreLoad() {
	if addr := c.Storage.Cache.Redis.Addr; addr != "" {
		username := c.Storage.Cache.Redis.Username
		password := c.Storage.Cache.Redis.Password
		if c.Util.Captcha.CacheType == "redis" &&
			c.Util.Captcha.Redis.Addr == "" {
			c.Util.Captcha.Redis.Addr = addr
			c.Util.Captcha.Redis.Username = username
			c.Util.Captcha.Redis.Password = password
		}
		if c.Middleware.RateLimiter.Store.Type == "redis" &&
			c.Middleware.RateLimiter.Store.Redis.Addr == "" {
			c.Middleware.RateLimiter.Store.Redis.Addr = addr
			c.Middleware.RateLimiter.Store.Redis.Username = username
			c.Middleware.RateLimiter.Store.Redis.Password = password
		}
		if c.Middleware.Auth.Store.Type == "redis" &&
			c.Middleware.Auth.Store.Redis.Addr == "" {
			c.Middleware.Auth.Store.Redis.Addr = addr
			c.Middleware.Auth.Store.Redis.Username = username
			c.Middleware.Auth.Store.Redis.Password = password
		}
	}
}

func (c *Config) Print() {
	if c.General.DisablePrintConfig {
		return
	}
	fmt.Println("// ----------------------- Load configurations start ------------------------")
	fmt.Println(c.String())
	fmt.Println("// ----------------------- Load configurations end --------------------------")
}

func (c *Config) FormatTableName(name string) string {
	return c.Storage.DB.TablePrefix + name
}
