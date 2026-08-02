package api

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
)

func NewToken() string {
	b := make([]byte, 24)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// Kept beside the machine's own configuration rather than inside the directory
// being received into, which is Foundry's and should hold nothing of ours.
func tokenPath(destination string) (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	abs, err := filepath.Abs(destination)
	if err != nil {
		abs = destination
	}
	sum := sha256.Sum256([]byte(abs))
	return filepath.Join(dir, "fvtt-migrate", "token-"+hex.EncodeToString(sum[:8])), nil
}

// StoredToken returns the key this destination has been using, creating and
// saving one the first time. A regenerated key on every restart would silently
// lock out the machine that was sending.
func StoredToken(destination string) (token string, fresh bool, err error) {
	path, err := tokenPath(destination)
	if err != nil {
		return "", false, err
	}

	if raw, err := os.ReadFile(path); err == nil {
		if existing := strings.TrimSpace(string(raw)); existing != "" {
			return existing, false, nil
		}
	}

	token = NewToken()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return token, true, err
	}
	if err := os.WriteFile(path, []byte(token+"\n"), 0o600); err != nil {
		return token, true, err
	}
	return token, true, nil
}
