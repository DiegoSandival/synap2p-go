package quicnet

import (
	"bytes"
	"context"
	"testing"
	"time"
)

func TestNewNodeClientProcessesOpcodes(t *testing.T) {
	t.Parallel()

	node, cancel := newTestNode(t, Config{
		Mode:       ModeClient,
		ListenPort: 0,
		DataDir:    t.TempDir(),
		Namespace:  "_synap2p_test_relays",
	})
	defer cancel()
	defer func() { _ = node.Close() }()

	requestID := []byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}
	if err := node.SendOpcode(0x0D, requestID, []byte("hello")); err != nil {
		t.Fatalf("send opcode: %v", err)
	}

	select {
	case event := <-node.Events():
		if len(event) < 21 {
			t.Fatalf("event too short: %d", len(event))
		}
		if event[3] != 0x0D {
			t.Fatalf("unexpected opcode: got 0x%02X want 0x0D", event[3])
		}
		if event[20] != 0x01 {
			t.Fatalf("unexpected status byte: got 0x%02X want 0x01", event[20])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for client event")
	}
}

func TestNewNodeServerRejectsOpcodeProcessing(t *testing.T) {
	t.Parallel()

	node, cancel := newTestNode(t, Config{
		Mode:       ModeServer,
		ListenPort: 0,
		DataDir:    t.TempDir(),
		Namespace:  "_synap2p_test_relays",
	})
	defer cancel()
	defer func() { _ = node.Close() }()

	if err := node.Process(make([]byte, 20)); err == nil {
		t.Fatal("expected server-only node to reject opcode processing")
	}
}

func TestNewNodeHybridProcessesOpcodes(t *testing.T) {
	t.Parallel()

	node, cancel := newTestNode(t, Config{
		Mode:       ModeHybrid,
		ListenPort: 0,
		DataDir:    t.TempDir(),
		Namespace:  "_synap2p_test_relays",
	})
	defer cancel()
	defer func() { _ = node.Close() }()

	if node.Mode() != ModeHybrid {
		t.Fatalf("unexpected mode: got %q want %q", node.Mode(), ModeHybrid)
	}

	requestID := []byte{15, 14, 13, 12, 11, 10, 9, 8, 7, 6, 5, 4, 3, 2, 1, 0}
	if err := node.SendOpcode(0x0D, requestID, []byte("hybrid")); err != nil {
		t.Fatalf("send opcode: %v", err)
	}

	select {
	case event := <-node.Events():
		if len(event) < 21 || event[3] != 0x0D || event[20] != 0x01 {
			t.Fatalf("unexpected hybrid event: %v", event)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for hybrid event")
	}
}

func TestNodeHelpersGenerateCID(t *testing.T) {
	t.Parallel()

	node, cancel := newTestNode(t, Config{
		Mode:       ModeClient,
		ListenPort: 0,
		DataDir:    t.TempDir(),
		Namespace:  "_synap2p_test_relays",
	})
	defer cancel()
	defer func() { _ = node.Close() }()

	requestID, err := node.GenerateCID([]byte("helper-cid"))
	if err != nil {
		t.Fatalf("generate cid helper: %v", err)
	}
	if len(requestID) != 16 {
		t.Fatalf("unexpected request ID length: got %d want 16", len(requestID))
	}

	select {
	case event := <-node.Events():
		if len(event) < 21 {
			t.Fatalf("event too short: %d", len(event))
		}
		if event[3] != OpcodeGenerateCID {
			t.Fatalf("unexpected opcode: got 0x%02X want 0x%02X", event[3], OpcodeGenerateCID)
		}
		if !bytes.Equal(event[4:20], requestID) {
			t.Fatalf("response ID mismatch: got %v want %v", event[4:20], requestID)
		}
		if event[20] != 0x01 {
			t.Fatalf("unexpected status byte: got 0x%02X want 0x01", event[20])
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for helper response")
	}
}

func newTestNode(t *testing.T, cfg Config) (*Node, context.CancelFunc) {
	t.Helper()

	node, err := NewNode(cfg)
	if err != nil {
		t.Fatalf("new node: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	if err := node.Start(ctx); err != nil {
		cancel()
		_ = node.Close()
		t.Fatalf("start node: %v", err)
	}

	return node, cancel
}
