package main

import (
	"encoding/base64"
	"fmt"
	"io"
	"strings"
	"testing"
)

func TestFriendlyHandshakeEOF(t *testing.T) {
	err := friendlyDesktopError(fmt.Errorf("ssh: handshake failed: %w", io.EOF))
	if !strings.Contains(err.Error(), "握手") {
		t.Fatal(err)
	}
	if !handshakeClosed(fmt.Errorf("ssh: handshake failed: %w", io.EOF)) {
		t.Fatal("eof not detected")
	}
}

func TestEncodeTerminalChunkPreservesPartialUTF8(t *testing.T) {
	raw := []byte{0x1b, '[', '3', '1', 'm', 0xe4, 0xb8}
	decoded, err := base64.StdEncoding.DecodeString(encodeTerminalChunk(raw))
	if err != nil {
		t.Fatal(err)
	}
	if string(decoded) != string(raw) {
		t.Fatalf("decoded %v", decoded)
	}
}
