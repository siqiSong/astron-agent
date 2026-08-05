package artifact

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

type fixedResolver struct{ ips []net.IPAddr }

func (resolver fixedResolver) LookupIPAddr(context.Context, string) ([]net.IPAddr, error) {
	return resolver.ips, nil
}

func TestFetchRejectsUnlistedAndPrivateHosts(t *testing.T) {
	policy := FetchPolicy{AllowedHosts: []string{"files.example"}, Resolver: fixedResolver{ips: []net.IPAddr{{IP: net.ParseIP("127.0.0.1")}}}, MaxBytes: 1024}
	if _, err := Fetch(context.Background(), "https://other.example/a.xlsx", policy); !errors.Is(err, ErrHostNotAllowed) {
		t.Fatalf("unlisted err=%v", err)
	}
	if _, err := Fetch(context.Background(), "https://files.example/a.xlsx", policy); !errors.Is(err, ErrPrivateAddress) {
		t.Fatalf("private err=%v", err)
	}
}

func TestFetchCapsBodyAndAcceptsCSV(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("a,b\n1,2\n")) }))
	defer server.Close()
	policy := FetchPolicy{AllowedHosts: []string{"127.0.0.1"}, AllowPrivate: true, MaxBytes: 7}
	if _, err := Fetch(context.Background(), server.URL+"/a.csv", policy); !errors.Is(err, ErrDownloadLimit) {
		t.Fatalf("err=%v", err)
	}
	policy.MaxBytes = 1024
	got, err := Fetch(context.Background(), server.URL+"/a.csv", policy)
	if err != nil || got.Format != "csv" {
		t.Fatalf("got=%+v err=%v", got, err)
	}
}

func TestFetchRejectsPrivateRedirect(t *testing.T) {
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "http://127.0.0.1/private.csv", http.StatusFound)
	}))
	defer redirect.Close()
	policy := FetchPolicy{AllowedHosts: []string{"127.0.0.1"}, AllowPrivate: true, MaxBytes: 1024, RedirectAllowPrivate: false}
	_, err := Fetch(context.Background(), redirect.URL, policy)
	if !errors.Is(err, ErrPrivateAddress) {
		t.Fatalf("err=%v", err)
	}
}
