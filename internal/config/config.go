package config

import (
	"strconv"
	"time"

	"github.com/byte-v-forge/common-lib/envx"
	"github.com/byte-v-forge/wa-app/internal/app"
)

type Config struct {
	ListenAddr         string
	DashboardHTTPAddr  string
	DashboardStaticDir string
	N8NWebhookBaseURL  string
	ProxyRuntimeAPIURL string
	PlatformNATSURL    string
	PGDSN              string
	RedisURL           string
	Engine             app.NativeEngineConfig
}

func Load() Config {
	return Config{
		ListenAddr:         envx.StringDefault("WA_APP_LISTEN_ADDR", ":50091"),
		DashboardHTTPAddr:  envx.StringDefault("WA_APP_DASHBOARD_HTTP_ADDR", ":8080"),
		DashboardStaticDir: envx.StringDefault("WA_APP_DASHBOARD_STATIC_DIR", "/app/dashboard/wa"),
		N8NWebhookBaseURL:  envx.StringDefault("WA_N8N_WEBHOOK_BASE_URL", ""),
		ProxyRuntimeAPIURL: envx.StringDefault("PROXY_RUNTIME_API_BASE_URL", ""),
		PlatformNATSURL:    envx.StringDefault("PLATFORM_NATS_URL", ""),
		PGDSN:              envx.StringDefault("WA_APP_PG_DSN", envx.StringDefault("PG_DSN", "")),
		RedisURL:           envx.StringDefault("WA_APP_REDIS_URL", envx.StringDefault("PLATFORM_REDIS_URL", "")),
		Engine: app.NativeEngineConfig{
			StateRoot:          envx.StringDefault("WA_APP_STATE_DIR", "var/wa-app/profiles"),
			ExistURL:           envx.StringDefault("WA_APP_EXIST_URL", ""),
			CodeURL:            envx.StringDefault("WA_APP_CODE_URL", ""),
			RegisterURL:        envx.StringDefault("WA_APP_REGISTER_URL", ""),
			ServerPublicKeyHex: envx.StringDefault("WA_APP_WASAFE_SERVER_PUBLIC_KEY_HEX", ""),
			RegistrationToken:  envx.StringDefault("WA_APP_REGISTRATION_TOKEN", ""),
			UserAgent:          envx.StringDefault("WA_APP_USER_AGENT", ""),
			AppVersion:         envx.StringDefault("WA_APP_VERSION", "2.26.21.73"),
			ProxyURL:           envx.StringDefault("WA_APP_PROXY_URL", ""),
			HTTPTimeout:        time.Duration(intEnv("WA_APP_HTTP_TIMEOUT_SECONDS", 20)) * time.Second,
			InsecureTLS:        boolEnv("WA_APP_INSECURE_TLS", true),
			ChatdHost:          envx.StringDefault("WA_APP_CHATD_HOST", "v.whatsapp.net"),
			ChatdPort:          intEnv("WA_APP_CHATD_PORT", 443),
			ChatdTLS:           boolEnv("WA_APP_CHATD_TLS", true),
			ChatdRoutingInfo:   envx.StringDefault("WA_APP_CHATD_ROUTING_INFO", ""),
			ChatdTimeout:       time.Duration(intEnv("WA_APP_CHATD_TIMEOUT_SECONDS", 15)) * time.Second,
			ChatdMaxFrameBytes: intEnv("WA_APP_CHATD_MAX_FRAME_BYTES", 4194304),
		},
	}
}

func intEnv(key string, fallback int) int {
	value := envx.StringDefault(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return parsed
}

func boolEnv(key string, fallback bool) bool {
	value := envx.StringDefault(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
