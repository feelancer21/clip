package main

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/nbd-wtf/go-nostr"
	"github.com/nbd-wtf/go-nostr/keyer"
	"github.com/nbd-wtf/go-nostr/nip19"
)

func loadKeyer(ctx context.Context, path string) (nostr.Keyer, error) {
	nsec, err := loadPrivateKeyPlain(path)
	if err != nil {
		return nil, err
	}
	return keyer.New(ctx, nil, nsec, nil)
}

func saveNsec(path string, nsec string) error {
	// ensure dir exists (0700)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		f.Close()
		os.Remove(tmp)
	}()

	if _, err := f.WriteString(nsec + "\n"); err != nil {
		return err
	}
	if err := f.Sync(); err != nil {
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}

	// atomic rename on same filesystem
	return os.Rename(tmp, path)
}

// LoadPrivateKeyPlain reads the hex private key from path. It is expected that the
// file contains only the nsec string in plain text.
func loadPrivateKeyPlain(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return "", fmt.Errorf("key file not found: %s", path)
		}
		return "", err
	}

	s := strings.TrimSpace(string(b))
	if s == "" {
		return "", errors.New("empty key file")
	}

	prefix, value, err := nip19.Decode(s)
	if err != nil {
		return "", err
	}

	if prefix != "nsec" {
		return "", fmt.Errorf("unexpected key prefix: got %s, want nsec", prefix)
	}

	if v, ok := value.(string); ok {
		return v, nil
	}

	return "", errors.New("invalid nsec format")
}

// DefaultKeyPath returns a reasonable per-user path like
//
//	Linux/macOS: $XDG_CONFIG_HOME/.<app>/key
func defaultKeyPath() (string, error) {
	return configDirFilePath("key")
}
