package identity

import (
	"path/filepath"
	"testing"

	"github.com/libp2p/go-libp2p/core/peer"
)

func TestLoadOrGenerateKeyPersistsPeerID(t *testing.T) {
	t.Parallel()

	keyPath := filepath.Join(t.TempDir(), "peer_identity.key")

	firstKey, err := LoadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("first key generation failed: %v", err)
	}
	firstID, err := peer.IDFromPrivateKey(firstKey)
	if err != nil {
		t.Fatalf("derive first peer ID: %v", err)
	}

	secondKey, err := LoadOrGenerateKey(keyPath)
	if err != nil {
		t.Fatalf("second key load failed: %v", err)
	}
	secondID, err := peer.IDFromPrivateKey(secondKey)
	if err != nil {
		t.Fatalf("derive second peer ID: %v", err)
	}

	if firstID != secondID {
		t.Fatalf("expected persistent peer ID, got %s and %s", firstID, secondID)
	}
}
