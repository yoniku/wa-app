package config

import (
	"github.com/byte-v-forge/common-lib/envx"
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
}

func Load() Config {
	return Config{
		ListenAddr:         envx.StringDefault("WA_APP_LISTEN_ADDR", ":50091"),
		DashboardHTTPAddr:  envx.StringDefault("WA_APP_DASHBOARD_HTTP_ADDR", ":8080"),
		DashboardStaticDir: envx.StringDefault("WA_APP_DASHBOARD_STATIC_DIR", "/app/dashboard/wa"),
		N8NWebhookBaseURL:  envx.StringDefault("WA_N8N_WEBHOOK_BASE_URL", ""),
		ProxyRuntimeAPIURL: envx.StringDefault("PROXY_RUNTIME_API_BASE_URL", ""),
		PlatformNATSURL:    envx.StringDefault("PLATFORM_NATS_URL", ""),
		PGDSN:              envx.StringDefault("WA_APP_PG_DSN", ""),
		RedisURL:           envx.StringDefault("PLATFORM_REDIS_URL", ""),
	}
}
