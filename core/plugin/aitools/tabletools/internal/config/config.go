package config

import (
	"os"
	"sort"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port                 int
	PublicBaseURL        string
	ArtifactDir          string
	ArtifactTTL          time.Duration
	MaxDownloadBytes     int64
	MaxExpandedBytes     int64
	MaxRows              int
	MaxColumns           int
	MaxChartPoints       int
	AllowedDownloadHosts []string
	ReadHeaderTimeout    time.Duration
	ReadTimeout          time.Duration
	WriteTimeout         time.Duration
	IdleTimeout          time.Duration
	ShutdownTimeout      time.Duration
}

func Default() Config {
	return Config{
		Port:              18669,
		PublicBaseURL:     "http://localhost/aitools-files",
		ArtifactDir:       "/opt/core/tabletools-data",
		ArtifactTTL:       24 * time.Hour,
		MaxDownloadBytes:  50 << 20,
		MaxExpandedBytes:  256 << 20,
		MaxRows:           100_000,
		MaxColumns:        1_024,
		MaxChartPoints:    10_000,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		ShutdownTimeout:   10 * time.Second,
	}
}

func Load() Config {
	cfg := Default()
	cfg.Port = envInt("TABLETOOLS_PORT", cfg.Port)
	publicBaseURL := strings.TrimSpace(os.Getenv("TABLETOOLS_PUBLIC_BASE_URL"))
	if publicBaseURL == "" {
		if consoleDomain := strings.TrimRight(strings.TrimSpace(os.Getenv("CONSOLE_DOMAIN")), "/"); consoleDomain != "" {
			publicBaseURL = consoleDomain + "/aitools-files"
		}
	}
	if publicBaseURL != "" {
		cfg.PublicBaseURL = strings.TrimRight(publicBaseURL, "/")
	}
	cfg.ArtifactDir = envString("TABLETOOLS_ARTIFACT_DIR", cfg.ArtifactDir)
	cfg.ArtifactTTL = envDuration("TABLETOOLS_ARTIFACT_TTL", cfg.ArtifactTTL)
	cfg.MaxDownloadBytes = envInt64("TABLETOOLS_MAX_DOWNLOAD_BYTES", cfg.MaxDownloadBytes)
	cfg.MaxExpandedBytes = envInt64("TABLETOOLS_MAX_EXPANDED_BYTES", cfg.MaxExpandedBytes)
	cfg.MaxRows = envInt("TABLETOOLS_MAX_ROWS", cfg.MaxRows)
	cfg.MaxColumns = envInt("TABLETOOLS_MAX_COLUMNS", cfg.MaxColumns)
	cfg.MaxChartPoints = envInt("TABLETOOLS_MAX_CHART_POINTS", cfg.MaxChartPoints)
	cfg.AllowedDownloadHosts = envHosts("TABLETOOLS_ALLOWED_DOWNLOAD_HOSTS")
	return cfg
}

func envString(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func envInt(key string, fallback int) int {
	value, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envInt64(key string, fallback int64) int64 {
	value, err := strconv.ParseInt(strings.TrimSpace(os.Getenv(key)), 10, 64)
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key)))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func envHosts(key string) []string {
	seen := make(map[string]struct{})
	for _, part := range strings.Split(os.Getenv(key), ",") {
		host := strings.ToLower(strings.TrimSpace(part))
		if host != "" {
			seen[host] = struct{}{}
		}
	}
	hosts := make([]string, 0, len(seen))
	for host := range seen {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	return hosts
}
