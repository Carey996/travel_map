// Package config 从环境变量读取全部配置。
//
// 与 Python 版一一对应，只是把 pydantic-settings 换成了显式解析：
// 每个字段都有默认值，因此本地开发复制 .env.example 即可直接跑通。
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config 汇总所有运行期配置。
type Config struct {
	AppName     string
	AppVersion  string
	Environment string
	APIPort     int

	DatabaseURL string

	RedisURL       string
	SessionBackend string // redis | db

	SessionCookieName string
	SessionTTL        time.Duration
	CookieSecure      bool
	CookieSameSite    string
	CSRFHeaderName    string
	CSRFHeaderValue   string

	CORSOrigins []string

	S3Endpoint       string
	S3PublicEndpoint string
	S3AccessKey      string
	S3SecretKey      string
	S3Bucket         string
	S3Region         string
	S3UseSSL         bool
	S3PresignExpires time.Duration

	GeocodeProvider    string // photon | nominatim
	PhotonBaseURL      string
	NominatimBaseURL   string
	NominatimUserAgent string
	NominatimLanguage  string
	GeocodeTimeout     time.Duration
}

// IsDev 报告是否运行在开发环境。
func (c *Config) IsDev() bool { return c.Environment == "dev" }

// BrowserEndpoint 返回浏览器直传使用的对象存储地址。
func (c *Config) BrowserEndpoint() string {
	if c.S3PublicEndpoint != "" {
		return c.S3PublicEndpoint
	}
	return c.S3Endpoint
}

// GeoBaseURL 返回当前地名服务的基地址。
func (c *Config) GeoBaseURL() string {
	if c.GeocodeProvider == "nominatim" {
		return c.NominatimBaseURL
	}
	return c.PhotonBaseURL
}

// Load 读取 .env（若存在）并构造配置。
//
// .env 放在仓库根目录；这里从当前工作目录向上找，避免依赖启动位置。
func Load() (*Config, error) {
	loadDotEnv()

	cfg := &Config{
		AppName:     env("APP_NAME", "Travel Blog API"),
		AppVersion:  env("APP_VERSION", "0.2.0"),
		Environment: env("ENVIRONMENT", "dev"),
		APIPort:     envInt("API_PORT", 8000),

		DatabaseURL: env("DATABASE_URL", "postgres://travelblog:travelblog@localhost:5432/travelblog?sslmode=disable"),

		RedisURL:       env("REDIS_URL", "redis://localhost:6379/0"),
		SessionBackend: env("SESSION_BACKEND", "redis"),

		SessionCookieName: env("SESSION_COOKIE_NAME", "tb_session"),
		SessionTTL:        time.Duration(envInt("SESSION_TTL_SECONDS", 60*60*24*30)) * time.Second,
		CookieSecure:      envBool("COOKIE_SECURE", false),
		CookieSameSite:    env("COOKIE_SAMESITE", "lax"),
		CSRFHeaderName:    env("CSRF_HEADER_NAME", "x-requested-with"),
		CSRFHeaderValue:   env("CSRF_HEADER_VALUE", "XMLHttpRequest"),

		CORSOrigins: envList("CORS_ORIGINS", []string{"http://localhost:5173", "http://127.0.0.1:5173"}),

		S3Endpoint:       env("S3_ENDPOINT_URL", "http://localhost:9000"),
		S3PublicEndpoint: env("S3_PUBLIC_ENDPOINT_URL", ""),
		S3AccessKey:      env("S3_ACCESS_KEY", "minioadmin"),
		S3SecretKey:      env("S3_SECRET_KEY", "minioadmin"),
		S3Bucket:         env("S3_BUCKET", "travelblog"),
		S3Region:         env("S3_REGION", "us-east-1"),
		S3UseSSL:         envBool("S3_USE_SSL", false),
		S3PresignExpires: time.Duration(envInt("S3_PRESIGN_EXPIRES", 3600)) * time.Second,

		GeocodeProvider:    env("GEOCODE_PROVIDER", "photon"),
		PhotonBaseURL:      env("PHOTON_BASE_URL", "https://photon.komoot.io"),
		NominatimBaseURL:   env("NOMINATIM_BASE_URL", "https://nominatim.openstreetmap.org"),
		NominatimUserAgent: env("NOMINATIM_USER_AGENT", "travel-blog/0.2 (personal project)"),
		NominatimLanguage:  env("NOMINATIM_LANGUAGE", "zh-CN,zh,en"),
		GeocodeTimeout:     time.Duration(envInt("GEOCODE_TIMEOUT_SECONDS", 8)) * time.Second,
	}

	// 归一化连接串：驱动前缀 + localhost 解析差异，详见两个函数的注释
	cfg.DatabaseURL = normalizeDatabaseURL(cfg.DatabaseURL)
	cfg.RedisURL = normalizeLocalhost(cfg.RedisURL)

	if cfg.SessionBackend != "redis" && cfg.SessionBackend != "db" {
		return nil, fmt.Errorf("SESSION_BACKEND 必须是 redis 或 db，当前为 %q", cfg.SessionBackend)
	}
	return cfg, nil
}

// loadDotEnv 从当前目录向上最多 4 层查找 .env。
func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for i := 0; i < 4; i++ {
		candidate := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(candidate); statErr == nil {
			_ = godotenv.Load(candidate)
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return
		}
		dir = parent
	}
}

func env(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && strings.TrimSpace(v) != "" {
		return strings.TrimSpace(v)
	}
	return fallback
}

func envInt(key string, fallback int) int {
	if v, ok := os.LookupEnv(key); ok {
		if n, err := strconv.Atoi(strings.TrimSpace(v)); err == nil {
			return n
		}
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	if v, ok := os.LookupEnv(key); ok {
		if b, err := strconv.ParseBool(strings.TrimSpace(v)); err == nil {
			return b
		}
	}
	return fallback
}

func envList(key string, fallback []string) []string {
	v, ok := os.LookupEnv(key)
	if !ok || strings.TrimSpace(v) == "" {
		return fallback
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

// normalizeDatabaseURL 把常见的驱动前缀归一化成 pgx 能识别的标准形式。
//
// Python 版用 SQLAlchemy，连接串写成 postgresql+asyncpg://...；
// Go 版用 pgx，只认 postgres:// 或 postgresql://。归一化后两边可以共用一份 .env。
func normalizeDatabaseURL(raw string) string {
	replacer := strings.NewReplacer(
		"postgresql+asyncpg://", "postgres://",
		"postgresql+psycopg://", "postgres://",
		"postgresql+psycopg2://", "postgres://",
		"postgresql://", "postgres://",
	)
	normalized := replacer.Replace(strings.TrimSpace(raw))
	// pgx 默认要求显式声明 sslmode；本地 MinIO/Postgres 场景下不带 TLS
	if !strings.Contains(normalized, "sslmode=") {
		if strings.Contains(normalized, "?") {
			normalized += "&sslmode=disable"
		} else {
			normalized += "?sslmode=disable"
		}
	}
	return normalized
}

// normalizeLocalhost 把连接串里的 localhost 换成 127.0.0.1。
//
// Windows 上 localhost 常被优先解析为 IPv6 的 ::1，而 Podman / Docker 的端口映射
// 只监听 IPv4，于是出现「容器明明在跑，连接却被拒」这种很容易误判的现象。
func normalizeLocalhost(raw string) string {
	return strings.ReplaceAll(raw, "//localhost:", "//127.0.0.1:")
}
