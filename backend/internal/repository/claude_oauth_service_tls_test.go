package repository

import (
	"crypto/tls"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateClaudeCodeReqClientUsesBunFingerprint(t *testing.T) {
	client, err := createClaudeCodeReqClient("")
	require.NoError(t, err)
	require.NotNil(t, client)

	httpClient := client.GetClient()
	require.NotNil(t, httpClient)
	transport, ok := httpClient.Transport.(*http.Transport)
	require.True(t, ok, "expected an *http.Transport for Claude Code TLS fingerprinting")
	require.NotNil(t, transport.DialTLSContext, "expected a custom TLS dialer")
	require.Nil(t, transport.TLSClientConfig, "crypto/tls configuration must be owned by the uTLS dialer")
	require.Equal(t, tls.VersionTLS13, 0x0304)
}
