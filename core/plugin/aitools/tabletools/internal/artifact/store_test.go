package artifact

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestStoreWriteOpenAndRejectTraversal(t *testing.T) {
	store, err := NewStore(t.TempDir(), "/aitools-files", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.Write("svg", strings.NewReader("<svg/>"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(item.URL, "/aitools-files/") {
		t.Fatalf("url=%q", item.URL)
	}
	file, err := store.Open(item.ID)
	if err != nil {
		t.Fatal(err)
	}
	file.Close()
	if _, err := store.Open("../secret"); !errors.Is(err, ErrInvalidArtifactID) {
		t.Fatalf("err=%v", err)
	}
}

func TestStoreWriteExposesStablePathAndAbsoluteURL(t *testing.T) {
	store, err := NewStore(t.TempDir(), "https://agent.example.com/aitools-files", time.Hour)
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.Write("svg", strings.NewReader("<svg/>"))
	if err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(item)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(encoded, &fields); err != nil {
		t.Fatal(err)
	}
	wantPath := "/aitools-files/" + item.ID
	if fields["path"] != wantPath {
		t.Fatalf("path=%v want=%q json=%s", fields["path"], wantPath, encoded)
	}
	if fields["url"] != "https://agent.example.com"+wantPath {
		t.Fatalf("url=%v want=%q json=%s", fields["url"], "https://agent.example.com"+wantPath, encoded)
	}
}

func TestStoreCleanupExpiresOnlyGeneratedArtifacts(t *testing.T) {
	store, err := NewStore(t.TempDir(), "/files", time.Nanosecond)
	if err != nil {
		t.Fatal(err)
	}
	item, err := store.Write("xlsx", strings.NewReader("book"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Cleanup(time.Now().Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Open(item.ID); !errors.Is(err, ErrArtifactNotFound) {
		t.Fatalf("err=%v", err)
	}
}

func TestStoreRejectsUnknownExtension(t *testing.T) {
	store, _ := NewStore(t.TempDir(), "/files", time.Hour)
	if _, err := store.Write("html", strings.NewReader("x")); !errors.Is(err, ErrInvalidArtifactID) {
		t.Fatalf("err=%v", err)
	}
}
