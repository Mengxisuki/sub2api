//go:build unit

package tlsfingerprint

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net"
	"strings"
	"testing"

	utls "github.com/refraction-networking/utls"
)

// capturedClaudeCodeBunJA3 是真实 Claude Code 2.1.220（Bun 1.4.0，macOS arm64）
// 在本机 tls-fingerprint 抓包得到的 JA3 哈希与完整 JA3 字符串。
const (
	capturedClaudeCodeBunJA3     = "d871d02cecbde59abbf8f4806134addf"
	capturedClaudeCodeBunJA3Text = "771,4865-4866-4867-49195-49199-49196-49200-52393-52392-49161-49171-49162-49172-156-157-47-53,0-23-65281-10-11-35-16-5-13-18-51-45-43-21,29-23-24,0"
)

// parsedClientHello 保存从序列化 ClientHello 中解析出的 JA3/JA4 所需字段。
type parsedClientHello struct {
	legacyVersion uint16
	ciphers       []uint16
	extensions    []uint16
	curves        []uint16
	pointFormats  []uint16
	alpn          []string
}

// parseClientHello 解析序列化后的 TLS ClientHello（utls 输出：1 字节 handshake
// type + 3 字节长度 + body）。
func parseClientHello(t *testing.T, raw []byte) parsedClientHello {
	t.Helper()
	if len(raw) < 9 || raw[0] != 0x01 {
		t.Fatalf("not a ClientHello (len=%d)", len(raw))
	}

	i := 4
	p := parsedClientHello{}
	p.legacyVersion = uint16(raw[i])<<8 | uint16(raw[i+1])
	i += 2 + 32 // legacy_version + random

	sidLen := int(raw[i])
	i += 1 + sidLen

	cipherLen := int(raw[i])<<8 | int(raw[i+1])
	i += 2
	for j := 0; j < cipherLen; j += 2 {
		p.ciphers = append(p.ciphers, uint16(raw[i+j])<<8|uint16(raw[i+j+1]))
	}
	i += cipherLen

	compLen := int(raw[i])
	i += 1 + compLen
	if i+2 > len(raw) {
		return p
	}

	extLen := int(raw[i])<<8 | int(raw[i+1])
	i += 2
	end := i + extLen
	for i+4 <= end {
		extType := uint16(raw[i])<<8 | uint16(raw[i+1])
		extDataLen := int(raw[i+2])<<8 | int(raw[i+3])
		i += 4
		extData := raw[i : i+extDataLen]
		p.extensions = append(p.extensions, extType)

		switch extType {
		case 10: // supported_groups
			for j := 2; j+1 < len(extData); j += 2 {
				p.curves = append(p.curves, uint16(extData[j])<<8|uint16(extData[j+1]))
			}
		case 11: // ec_point_formats
			n := int(extData[0])
			for j := 1; j <= n && j < len(extData); j++ {
				p.pointFormats = append(p.pointFormats, uint16(extData[j]))
			}
		case 16: // alpn
			for off := 2; off < len(extData); {
				l := int(extData[off])
				off++
				p.alpn = append(p.alpn, string(extData[off:off+l]))
				off += l
			}
		}
		i += extDataLen
	}
	return p
}

func (p parsedClientHello) ja3String() string {
	return fmt.Sprintf("%d,%s,%s,%s,%s",
		p.legacyVersion,
		joinUint16(p.ciphers, "-"),
		joinUint16(p.extensions, "-"),
		joinUint16(p.curves, "-"),
		joinUint16(p.pointFormats, "-"))
}

func (p parsedClientHello) ja3Hash() string {
	sum := md5.Sum([]byte(p.ja3String()))
	return hex.EncodeToString(sum[:])
}

// ja4Prefix 返回 JA4 前缀（不含 hash 部分），例如 t13d1714h1。
func (p parsedClientHello) ja4Prefix() string {
	alpnCode := "00"
	if len(p.alpn) > 0 {
		if p.alpn[0] == "h2" {
			alpnCode = "h2"
		} else {
			alpnCode = "h1"
		}
	}
	return fmt.Sprintf("t13d%02d%02d%s", len(p.ciphers), len(p.extensions), alpnCode)
}

func joinUint16(vals []uint16, sep string) string {
	parts := make([]string, len(vals))
	for i, v := range vals {
		parts[i] = fmt.Sprintf("%d", v)
	}
	return strings.Join(parts, sep)
}

// marshalProfile 序列化指定 Profile 的 ClientHello，返回原始字节。
func marshalProfile(t *testing.T, p *Profile) []byte {
	t.Helper()
	uconn := utls.UClient(&net.TCPConn{}, &utls.Config{ServerName: "api.anthropic.com"}, utls.HelloCustom)
	if err := uconn.ApplyPreset(buildClientHelloSpecFromProfile(p)); err != nil {
		t.Fatalf("ApplyPreset: %v", err)
	}
	if err := uconn.MarshalClientHello(); err != nil {
		t.Fatalf("MarshalClientHello: %v", err)
	}
	return uconn.HandshakeState.Hello.Raw
}

func equalUint16(a, b []uint16) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// TestClaudeCodeBunProfileMatchesCaptured verifies the built-in Claude Code (Bun)
// simulation mode reproduces the exact ClientHello captured from real Claude Code
// 2.1.220 (Bun 1.4.0) on this machine: padding (21) present, no ECH GREASE (65037).
func TestClaudeCodeBunProfileMatchesCaptured(t *testing.T) {
	raw := marshalProfile(t, ClaudeCodeBunProfile())
	ch := parseClientHello(t, raw)

	if got := ch.ja3Hash(); got != capturedClaudeCodeBunJA3 {
		t.Errorf("JA3 hash mismatch:\n  got:      %s\n  expected: %s\n  ja3:      %s", got, capturedClaudeCodeBunJA3, ch.ja3String())
	}
	if got := ch.ja3String(); got != capturedClaudeCodeBunJA3Text {
		t.Errorf("JA3 string mismatch:\n  got:      %s\n  expected: %s", got, capturedClaudeCodeBunJA3Text)
	}

	wantExt := []uint16{0, 23, 65281, 10, 11, 35, 16, 5, 13, 18, 51, 45, 43, 21}
	if !equalUint16(ch.extensions, wantExt) {
		t.Errorf("extension order mismatch:\n  got:      %v\n  expected: %v", ch.extensions, wantExt)
	}
	if len(ch.extensions) != 14 {
		t.Errorf("expected 14 extensions, got %d", len(ch.extensions))
	}
	if len(ch.ciphers) != 17 {
		t.Errorf("expected 17 cipher suites, got %d", len(ch.ciphers))
	}
	if ch.extensions[len(ch.extensions)-1] != 21 {
		t.Errorf("padding (21) must be the last extension, got %v", ch.extensions)
	}
	if hasValue(ch.extensions, 65037) {
		t.Error("ECH GREASE (65037) must be absent in Claude Code (Bun) mode")
	}
	if hasGREASE(ch.extensions) {
		t.Errorf("no GREASE extension expected, got %v", ch.extensions)
	}
	if len(ch.alpn) != 1 || ch.alpn[0] != "http/1.1" {
		t.Errorf("expected ALPN [http/1.1], got %v", ch.alpn)
	}
	if got := ch.ja4Prefix(); got != "t13d1714h1" {
		t.Errorf("JA4 prefix mismatch: got %s, expected t13d1714h1", got)
	}
}

// TestNodeJS24ProfileLegacyPreset guards the legacy Node.js 24.x preset so it
// still emits the old extension vector (ECH GREASE, no padding) when selected.
func TestNodeJS24ProfileLegacyPreset(t *testing.T) {
	raw := marshalProfile(t, NodeJS24Profile())
	ch := parseClientHello(t, raw)

	if got := ch.ja3Hash(); got != "44f88fca027f27bab4bb08d4af15f23e" {
		t.Errorf("JA3 hash mismatch: got %s, expected 44f88fca027f27bab4bb08d4af15f23e", got)
	}
	wantExt := []uint16{0, 65037, 23, 65281, 10, 11, 35, 16, 5, 13, 18, 51, 45, 43}
	if !equalUint16(ch.extensions, wantExt) {
		t.Errorf("extension order mismatch:\n  got:      %v\n  expected: %v", ch.extensions, wantExt)
	}
}

// TestClaudeCodeBunProfileUsesBoringPadding checks the spec uses a real
// BoringSSL-style padding extension rather than an empty generic one.
func TestClaudeCodeBunProfileUsesBoringPadding(t *testing.T) {
	spec := buildClientHelloSpecFromProfile(ClaudeCodeBunProfile())
	found := false
	for _, ext := range spec.Extensions {
		if pe, ok := ext.(*utls.UtlsPaddingExtension); ok {
			found = true
			if pe.GetPaddingLen == nil {
				t.Error("padding extension must use a padding-length functor (BoringSSL style)")
			}
		}
	}
	if !found {
		t.Error("expected UtlsPaddingExtension in Claude Code (Bun) profile spec")
	}
}

func hasValue(vals []uint16, want uint16) bool {
	for _, v := range vals {
		if v == want {
			return true
		}
	}
	return false
}

func hasGREASE(vals []uint16) bool {
	for _, v := range vals {
		if isGREASEValue(v) {
			return true
		}
	}
	return false
}
