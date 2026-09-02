//go:build unit

package tlsfingerprint

import (
	"testing"

	utls "github.com/refraction-networking/utls"
	"github.com/stretchr/testify/require"
)

func TestSessionCachesAreDialerScoped(t *testing.T) {
	first := NewDialer(ClaudeCodeBunProfile(), nil)
	second := NewDialer(ClaudeCodeBunProfile(), nil)

	require.NotSame(t, first.sessions, second.sessions)

	first.sessions.Put("api.anthropic.com", &utls.ClientSessionState{})
	session, ok := first.sessions.Get("api.anthropic.com")
	require.True(t, ok)
	require.NotNil(t, session)

	_, ok = second.sessions.Get("api.anthropic.com")
	require.False(t, ok)
}

func TestSessionCacheDeleteNil(t *testing.T) {
	cache := NewSynchronizedSessionCache()
	cache.Put("api.anthropic.com", &utls.ClientSessionState{})
	cache.Put("api.anthropic.com", nil)

	_, ok := cache.Get("api.anthropic.com")
	require.False(t, ok)
}
