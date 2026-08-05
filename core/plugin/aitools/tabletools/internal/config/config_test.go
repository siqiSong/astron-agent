package config

import (
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("TABLETOOLS_PORT", "")
	t.Setenv("TABLETOOLS_PUBLIC_BASE_URL", "")
	t.Setenv("TABLETOOLS_MAX_DOWNLOAD_BYTES", "")
	t.Setenv("TABLETOOLS_ARTIFACT_TTL", "")

	got := Load()
	if got.Port != 18669 {
		t.Fatalf("Port = %d, want 18669", got.Port)
	}
	if got.PublicBaseURL != "http://localhost/aitools-files" {
		t.Fatalf("PublicBaseURL = %q, want browser-reachable localhost URL", got.PublicBaseURL)
	}
	if got.MaxDownloadBytes != 50<<20 {
		t.Fatalf("MaxDownloadBytes = %d, want %d", got.MaxDownloadBytes, 50<<20)
	}
	if got.ArtifactTTL != 24*time.Hour {
		t.Fatalf("ArtifactTTL = %s, want 24h", got.ArtifactTTL)
	}
}

func TestLoadOverridesAndFiltersHosts(t *testing.T) {
	t.Setenv("TABLETOOLS_PORT", "19000")
	t.Setenv("TABLETOOLS_ARTIFACT_TTL", "2h")
	t.Setenv("TABLETOOLS_ALLOWED_DOWNLOAD_HOSTS", " minio:9000,Files.Example.test, ,minio:9000 ")

	got := Load()
	if got.Port != 19000 || got.ArtifactTTL != 2*time.Hour {
		t.Fatalf("unexpected overrides: %+v", got)
	}
	want := []string{"files.example.test", "minio:9000"}
	if len(got.AllowedDownloadHosts) != len(want) {
		t.Fatalf("AllowedDownloadHosts = %#v, want %#v", got.AllowedDownloadHosts, want)
	}
	for i := range want {
		if got.AllowedDownloadHosts[i] != want[i] {
			t.Fatalf("AllowedDownloadHosts = %#v, want %#v", got.AllowedDownloadHosts, want)
		}
	}
}

func TestLoadBuildsPublicBaseURLFromConsoleDomain(t *testing.T) {
	t.Setenv("TABLETOOLS_PUBLIC_BASE_URL", "")
	t.Setenv("CONSOLE_DOMAIN", "https://agent.example.com:8443/")

	got := Load()
	if got.PublicBaseURL != "https://agent.example.com:8443/aitools-files" {
		t.Fatalf("PublicBaseURL=%q", got.PublicBaseURL)
	}
}

func TestLoadExplicitPublicBaseURLTakesPrecedence(t *testing.T) {
	t.Setenv("CONSOLE_DOMAIN", "https://agent.example.com")
	t.Setenv("TABLETOOLS_PUBLIC_BASE_URL", "https://cdn.example.com/custom-files/")

	got := Load()
	if got.PublicBaseURL != "https://cdn.example.com/custom-files" {
		t.Fatalf("PublicBaseURL=%q", got.PublicBaseURL)
	}
}
