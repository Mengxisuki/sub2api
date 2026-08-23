package service

import (
	"bytes"
	"compress/gzip"
	"io"
	"math/rand"
	"testing"
)

func TestShouldGzipClaudeRequestBody(t *testing.T) {
	tests := []struct {
		name string
		body string
		url  string
		want bool
	}{
		{name: "below threshold", body: string(bytes.Repeat([]byte("x"), 4095)), url: "https://api.anthropic.com/v1/messages", want: false},
		{name: "official endpoint", body: string(bytes.Repeat([]byte("x"), 4096)), url: "https://api.anthropic.com/v1/messages", want: true},
		{name: "custom relay", body: string(bytes.Repeat([]byte("x"), 4096)), url: "https://relay.example/v1/messages", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldGzipClaudeRequestBody([]byte(tt.body), tt.url, false, false, false); got != tt.want {
				t.Fatalf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestGzipClaudeRequestBodyBunCompatible(t *testing.T) {
	rand.New(rand.NewSource(1))
	body := appendClaudeGzipWhitespaceTail(nil, []byte(`{"x":"AAA"}`))
	wire, err := gzipClaudeRequestBodyBunCompatible(body)
	if err != nil {
		t.Fatal(err)
	}
	if wire[0] != 0x1f || wire[1] != 0x8b || wire[2] != 8 || wire[3] != 0 || wire[8] != 0 || wire[9] != 0x13 {
		t.Fatalf("unexpected gzip header: % x", wire[:10])
	}
	reader, err := gzip.NewReader(bytes.NewReader(wire))
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(decoded, body) {
		t.Fatal("decoded body mismatch")
	}
	if body[0] != '{' || (body[len(body)-1] != ' ' && body[len(body)-1] != '\t') {
		t.Fatalf("unexpected padded body: %q", body)
	}
}
