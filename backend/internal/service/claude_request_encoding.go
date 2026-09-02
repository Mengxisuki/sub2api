package service

import (
	"bytes"
	"compress/flate"
	"encoding/binary"
	"hash/crc32"
	"math/rand"
	"net/url"
)

const claudeGzipRequestBodyMinChars = 4096

func shouldGzipClaudeRequestBody(body []byte, url string, hasProxy, hasMTLS, hasCustomCA bool) bool {
	if len(body) < claudeGzipRequestBodyMinChars {
		return false
	}
	// Claude Code 2.1.241 disables this behavior for its local-proxy, mTLS, and
	// custom CA paths. Custom relay/base URLs are treated like custom transports.
	if hasProxy || hasMTLS || hasCustomCA || isCustomClaudeTransportURL(url) {
		return false
	}
	return true
}

func isCustomClaudeTransportURL(rawURL string) bool {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return true
	}
	return parsed.Scheme != "https" || parsed.Host != "api.anthropic.com"
}

func appendClaudeGzipWhitespaceTail(dst, body []byte) []byte {
	dst = append(dst, body...)
	dst = append(dst, '\n')
	tail := rand.Intn(257)
	for i := 0; i < tail; i++ {
		if rand.Intn(2) == 0 {
			dst = append(dst, ' ')
		} else {
			dst = append(dst, '\t')
		}
	}
	return dst
}

func gzipClaudeRequestBodyBunCompatible(body []byte) ([]byte, error) {
	var deflateBody bytes.Buffer
	writer, err := flate.NewWriter(&deflateBody, flate.DefaultCompression)
	if err != nil {
		return nil, err
	}
	if _, err := writer.Write(body); err != nil {
		return nil, err
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}

	// Bun's gzip framing uses a zero-mtime header, zlib's raw-deflate output,
	// and the standard CRC32/ISIZE trailer. Go's compress/gzip uses a different
	// DEFLATE implementation, so its byte stream is not wire-equivalent.
	output := make([]byte, 0, deflateBody.Len()+18)
	output = append(output, 0x1f, 0x8b, 0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x13)
	output = append(output, deflateBody.Bytes()...)
	var trailer [8]byte
	binary.LittleEndian.PutUint32(trailer[:4], crc32.ChecksumIEEE(body))
	binary.LittleEndian.PutUint32(trailer[4:], uint32(len(body)))
	return append(output, trailer[:]...), nil
}
