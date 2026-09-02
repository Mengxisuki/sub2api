package repository

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/imroc/req/v3"
	"github.com/stretchr/testify/require"
)

func TestClaudeOAuthTokenOperationsUseCLITransport(t *testing.T) {
	var requestCount int32
	transport := roundTripFunc(func(r *http.Request) (*http.Response, error) {
		atomic.AddInt32(&requestCount, 1)
		recorder := httptest.NewRecorder()
		recorder.Header().Set("Content-Type", "application/json")
		recorder.WriteString(`{"access_token":"access-token","refresh_token":"refresh-token"}`)
		return recorder.Result(), nil
	})

	service, ok := NewClaudeOAuthClient().(*claudeOAuthService)
	require.True(t, ok)
	service.tokenURL = "https://claude.internal/token"

	webFactoryUsed := false
	service.clientFactory = func(string) (*req.Client, error) {
		webFactoryUsed = true
		return nil, http.ErrAbortHandler
	}
	service.cliOAuthClientFactory = func(string) (*req.Client, error) {
		return newTestReqClient(transport), nil
	}

	token, err := service.ExchangeCodeForToken(context.Background(), "code#state", "verifier", "", "", false)
	require.NoError(t, err)
	require.Equal(t, "access-token", token.AccessToken)

	token, err = service.RefreshToken(context.Background(), "refresh-token", "")
	require.NoError(t, err)
	require.Equal(t, "refresh-token", token.RefreshToken)

	require.False(t, webFactoryUsed)
	require.EqualValues(t, 2, atomic.LoadInt32(&requestCount))
}
