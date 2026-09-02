package claude

import (
	"strings"
)

const (
	ClientProfileClaudeCode241 = "claude_code_2_1_241"
	ClientProfileClaudeCode220 = "claude_code_2_1_220"
	DefaultClientProfileID     = ClientProfileClaudeCode241

	// AccountExtraKey stores the selected immutable client snapshot at account
	// level. Empty value means "use the server's default snapshot".
	AccountExtraKey = "claude_client_profile_id"
)

type GzipPolicy struct {
	Enabled   bool
	MinChars  int
	Fallbacks []int
}

type ClientProfile struct {
	ID                  string            `json:"id"`
	Name                string            `json:"name"`
	Description         string            `json:"description"`
	CLIVersion          string            `json:"cli_version"`
	SDKPackageVersion   string            `json:"sdk_package_version"`
	DefaultTLSMode      string            `json:"default_tls_mode"`
	Headers             map[string]string `json:"-"`
	MessageBetas        []string          `json:"-"`
	CountTokenBetas     []string          `json:"-"`
	Gzip                GzipPolicy        `json:"-"`
	BetaNames           []string          `json:"beta_names,omitempty"`
	SupportsRequestGzip bool              `json:"supports_request_gzip"`
	GzipMinChars        int               `json:"gzip_min_chars,omitempty"`
}

var clientProfiles = map[string]ClientProfile{
	ClientProfileClaudeCode241: {
		ID:                ClientProfileClaudeCode241,
		Name:              "Claude Code 2.1.241",
		Description:       "Verified Claude Code 2.1.241 application-layer snapshot.",
		CLIVersion:        CLICurrentVersion,
		SDKPackageVersion: "0.112.1",
		DefaultTLSMode:    "claude_code_bun",
		Headers:           DefaultHeaders,
		MessageBetas:      FullClaudeCodeMimicryBetas(),
		CountTokenBetas: append(
			FullClaudeCodeMimicryBetas(),
			BetaTokenCounting,
		),
		Gzip: GzipPolicy{
			Enabled:   true,
			MinChars:  4096,
			Fallbacks: []int{400, 403, 415},
		},
		SupportsRequestGzip: true,
		GzipMinChars:        4096,
	},
	ClientProfileClaudeCode220: {
		ID:                ClientProfileClaudeCode220,
		Name:              "Claude Code 2.1.220",
		Description:       "Frozen Claude Code 2.1.220 sdk-cli snapshot captured on macOS arm64.",
		CLIVersion:        "2.1.220",
		SDKPackageVersion: "0.94.0",
		DefaultTLSMode:    "claude_code_bun",
		Headers: map[string]string{
			"User-Agent":                                "claude-cli/2.1.220 (external, sdk-cli)",
			"X-Stainless-Lang":                          "js",
			"X-Stainless-Package-Version":               "0.94.0",
			"X-Stainless-OS":                            "MacOS",
			"X-Stainless-Arch":                          "arm64",
			"X-Stainless-Runtime":                       "node",
			"X-Stainless-Runtime-Version":               "v26.3.0",
			"X-Stainless-Retry-Count":                   "0",
			"X-Stainless-Timeout":                       "600",
			"X-App":                                     "cli",
			"Anthropic-Dangerous-Direct-Browser-Access": "true",
		},
		MessageBetas: FullClaudeCodeMimicryBetas(),
		CountTokenBetas: append(
			FullClaudeCodeMimicryBetas(),
			BetaTokenCounting,
		),
		SupportsRequestGzip: false,
	},
}

func GetClientProfile(id string) (*ClientProfile, bool) {
	if id == "" {
		id = DefaultClientProfileID
	}
	profile, ok := clientProfiles[id]
	if !ok {
		return nil, false
	}
	return &profile, true
}

func DefaultClientProfile() *ClientProfile {
	profile, _ := GetClientProfile(DefaultClientProfileID)
	return profile
}

func ListClientProfiles() []*ClientProfile {
	return []*ClientProfile{
		mustClientProfile(ClientProfileClaudeCode241),
		mustClientProfile(ClientProfileClaudeCode220),
	}
}

func mustClientProfile(id string) *ClientProfile {
	profile, ok := GetClientProfile(id)
	if !ok {
		panic("unknown built-in client profile: " + id)
	}
	return profile
}

func profileContains(items []string, wanted string) bool {
	for _, item := range items {
		if strings.EqualFold(strings.TrimSpace(item), wanted) {
			return true
		}
	}
	return false
}

func (p *ClientProfile) modelMessageBetas(modelID string) []string {
	if p == nil {
		p = DefaultClientProfile()
	}
	betas := make([]string, len(p.MessageBetas))
	copy(betas, p.MessageBetas)
	if !strings.Contains(strings.ToLower(modelID), "haiku") {
		return betas
	}

	filtered := make([]string, 0, len(betas))
	omitClaudeCode := true
	normalizedModel := strings.ToLower(modelID)
	isHaiku45 := strings.Contains(normalizedModel, "haiku-4-5")
	for _, beta := range betas {
		if omitClaudeCode && beta == BetaClaudeCode {
			continue
		}
		if isHaiku45 && (beta == BetaInterleavedThinking || beta == BetaEffort) {
			continue
		}
		if normalizedModel != "" && !isHaiku45 && beta == BetaEffort {
			continue
		}
		filtered = append(filtered, beta)
	}
	return filtered
}

func (p *ClientProfile) ModelMessageBetas(modelID string) []string {
	return p.modelMessageBetas(modelID)
}

func (p *ClientProfile) ModelCountTokenBetas(modelID string) []string {
	betas := p.ModelMessageBetas(modelID)
	if !profileContains(betas, BetaTokenCounting) {
		betas = append(betas, BetaTokenCounting)
	}
	return betas
}

func (p *ClientProfile) HeaderValue(key string) string {
	if p == nil || p.Headers == nil {
		return ""
	}
	return p.Headers[key]
}

func (p *ClientProfile) CanGzipRequestBody(bodyLength int) bool {
	if p == nil {
		p = DefaultClientProfile()
	}
	return p.Gzip.Enabled && bodyLength >= p.Gzip.MinChars
}
