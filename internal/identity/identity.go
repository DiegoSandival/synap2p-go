package identity

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	crypto "github.com/libp2p/go-libp2p/core/crypto"
)

func LoadOrGenerateKey(path string) (crypto.PrivKey, error) {
	if raw, err := os.ReadFile(path); err == nil {
		privKey, err := crypto.UnmarshalPrivateKey(raw)
		if err != nil {
			return nil, fmt.Errorf("unmarshal private key %q: %w", path, err)
		}
		return privKey, nil
	} else if !os.IsNotExist(err) {
		return nil, fmt.Errorf("read private key %q: %w", path, err)
	}

	privKey, _, err := crypto.GenerateEd25519Key(rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("generate Ed25519 identity: %w", err)
	}

	raw, err := crypto.MarshalPrivateKey(privKey)
	if err != nil {
		return nil, fmt.Errorf("marshal generated private key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil && filepath.Dir(path) != "." {
		return nil, fmt.Errorf("create identity directory for %q: %w", path, err)
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(path), "peer_identity_*.key")
	if err != nil {
		return nil, fmt.Errorf("create temp identity file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	if _, err := tmpFile.Write(raw); err != nil {
		_ = tmpFile.Close()
		return nil, fmt.Errorf("write identity file %q: %w", tmpPath, err)
	}
	if err := tmpFile.Close(); err != nil {
		return nil, fmt.Errorf("close identity file %q: %w", tmpPath, err)
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmpPath, 0o600); err != nil {
			return nil, fmt.Errorf("chmod identity file %q: %w", tmpPath, err)
		}
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return nil, fmt.Errorf("persist identity file %q: %w", path, err)
	}
	return privKey, nil
}