package claude

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetClientProfile(t *testing.T) {
	profile, ok := GetClientProfile(DefaultClientProfileID)
	require.True(t, ok)
	require.Equal(t, DefaultClientProfileID, profile.ID)
	require.Equal(t, CLICurrentVersion, profile.CLIVersion)
	require.Equal(t, DefaultHeaders["User-Agent"], profile.HeaderValue("User-Agent"))
	require.Equal(t, DefaultHeaders["X-Stainless-Package-Version"], profile.HeaderValue("X-Stainless-Package-Version"))
	require.ElementsMatch(t, FullClaudeCodeMimicryBetas(), profile.MessageBetas)
	require.True(t, profile.CanGzipRequestBody(4096))
	require.False(t, profile.CanGzipRequestBody(4095))
}

func TestLegacyClientProfileSnapshot(t *testing.T) {
	profile, ok := GetClientProfile(ClientProfileClaudeCode220)
	require.True(t, ok)
	require.Equal(t, "2.1.220", profile.CLIVersion)
	require.Equal(t, "0.94.0", profile.HeaderValue("X-Stainless-Package-Version"))
	require.Equal(t, "claude-cli/2.1.220 (external, sdk-cli)", profile.HeaderValue("User-Agent"))
	require.Equal(t, "MacOS", profile.HeaderValue("X-Stainless-OS"))
	require.Equal(t, "v26.3.0", profile.HeaderValue("X-Stainless-Runtime-Version"))
	require.False(t, profile.SupportsRequestGzip)
	require.False(t, profile.CanGzipRequestBody(1<<20))
}

func TestGetClientProfileInvalidFallsBackToDefault(t *testing.T) {
	defaultProfile := DefaultClientProfile()
	profile, ok := GetClientProfile("unknown-version")
	require.False(t, ok)
	require.Nil(t, profile)
	require.Equal(t, defaultProfile.ID, DefaultClientProfile().ID)
}

func TestModelMessageBetasByModel(t *testing.T) {
	profile := DefaultClientProfile()
	full := profile.ModelMessageBetas("claude-sonnet-4")
	require.Contains(t, full, BetaClaudeCode)
	require.Contains(t, full, BetaInterleavedThinking)

	haiku := profile.ModelMessageBetas("claude-haiku-4")
	require.NotContains(t, haiku, BetaClaudeCode)
	require.Contains(t, haiku, BetaInterleavedThinking)

	haiku45 := profile.ModelCountTokenBetas("claude-haiku-4-5")
	require.NotContains(t, haiku45, BetaClaudeCode)
	require.NotContains(t, haiku45, BetaInterleavedThinking)
	require.Contains(t, haiku45, BetaTokenCounting)
}
