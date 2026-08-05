package artifact

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

var (
	ErrInvalidArtifactID = errors.New("invalid artifact id")
	ErrArtifactNotFound  = errors.New("artifact not found")
	artifactPattern      = regexp.MustCompile(`^[a-f0-9]{32}\.(svg|xlsx)$`)
)

type Artifact struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	URL  string `json:"url"`
}

type Store struct {
	dir, publicBase string
	ttl             time.Duration
}

func NewStore(dir, publicBase string, ttl time.Duration) (*Store, error) {
	if strings.TrimSpace(dir) == "" || ttl <= 0 {
		return nil, ErrInvalidArtifactID
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	return &Store{dir: absolute, publicBase: strings.TrimRight(publicBase, "/"), ttl: ttl}, nil
}

func (store *Store) Write(extension string, reader io.Reader) (Artifact, error) {
	extension = strings.ToLower(strings.TrimPrefix(extension, "."))
	if extension != "svg" && extension != "xlsx" {
		return Artifact{}, ErrInvalidArtifactID
	}
	random := make([]byte, 16)
	if _, err := rand.Read(random); err != nil {
		return Artifact{}, err
	}
	id := hex.EncodeToString(random) + "." + extension
	temporary, err := os.CreateTemp(store.dir, ".artifact-*")
	if err != nil {
		return Artifact{}, err
	}
	temporaryName := temporary.Name()
	committed := false
	defer func() {
		temporary.Close()
		if !committed {
			os.Remove(temporaryName)
		}
	}()
	if _, err = io.Copy(temporary, reader); err != nil {
		return Artifact{}, err
	}
	if err = temporary.Sync(); err != nil {
		return Artifact{}, err
	}
	if err = temporary.Close(); err != nil {
		return Artifact{}, err
	}
	if err = os.Rename(temporaryName, filepath.Join(store.dir, id)); err != nil {
		return Artifact{}, err
	}
	committed = true
	path := "/aitools-files/" + id
	return Artifact{ID: id, Path: path, URL: store.publicBase + "/" + id}, nil
}

func (store *Store) Open(id string) (*os.File, error) {
	if !artifactPattern.MatchString(id) {
		return nil, ErrInvalidArtifactID
	}
	file, err := os.Open(filepath.Join(store.dir, id))
	if errors.Is(err, os.ErrNotExist) {
		return nil, ErrArtifactNotFound
	}
	return file, err
}

func (store *Store) Cleanup(now time.Time) error {
	entries, err := os.ReadDir(store.dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !artifactPattern.MatchString(entry.Name()) {
			continue
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return statErr
		}
		if now.Sub(info.ModTime()) > store.ttl {
			if removeErr := os.Remove(filepath.Join(store.dir, entry.Name())); removeErr != nil && !errors.Is(removeErr, os.ErrNotExist) {
				return fmt.Errorf("cleanup %s: %w", entry.Name(), removeErr)
			}
		}
	}
	return nil
}
