//go:build unit

package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/stretchr/testify/require"
)

type tlsProfileRepoStub struct {
	profiles []*model.TLSFingerprintProfile
}

func (r *tlsProfileRepoStub) List(context.Context) ([]*model.TLSFingerprintProfile, error) {
	return r.profiles, nil
}
func (r *tlsProfileRepoStub) GetByID(context.Context, int64) (*model.TLSFingerprintProfile, error) {
	return nil, nil
}
func (r *tlsProfileRepoStub) Create(_ context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	r.profiles = append(r.profiles, profile)
	return profile, nil
}
func (r *tlsProfileRepoStub) Update(context.Context, *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	return nil, nil
}
func (r *tlsProfileRepoStub) Delete(context.Context, int64) error { return nil }

type tlsProfileCacheStub struct{}

func (c *tlsProfileCacheStub) Get(context.Context) ([]*model.TLSFingerprintProfile, bool) {
	return nil, false
}
func (c *tlsProfileCacheStub) Set(context.Context, []*model.TLSFingerprintProfile) error {
	return nil
}
func (c *tlsProfileCacheStub) Invalidate(context.Context) error         { return nil }
func (c *tlsProfileCacheStub) NotifyUpdate(context.Context) error       { return nil }
func (c *tlsProfileCacheStub) SubscribeUpdates(context.Context, func()) {}

type tlsAccountRepoStub struct {
	updates []map[string]any
}

func (r *tlsAccountRepoStub) UpdateExtra(_ context.Context, _ int64, updates map[string]any) error {
	r.updates = append(r.updates, updates)
	return nil
}

func TestResolveTLSProfilePersistsRandomSelection(t *testing.T) {
	repo := &tlsProfileRepoStub{profiles: []*model.TLSFingerprintProfile{
		{ID: 10, Name: "one"},
		{ID: 20, Name: "two"},
	}}
	accountRepo := &tlsAccountRepoStub{}
	svc := NewTLSFingerprintProfileService(repo, &tlsProfileCacheStub{}, nil, accountRepo)
	account := &Account{
		ID:       99,
		Platform: PlatformAnthropic,
		Type:     AccountTypeOAuth,
		Extra:    map[string]any{"tls_fingerprint_profile_id": float64(-1), "enable_tls_fingerprint": true},
	}

	first := svc.ResolveTLSProfile(account)
	require.NotNil(t, first)
	second := svc.ResolveTLSProfile(account)
	require.Equal(t, first.Name, second.Name)
	require.Len(t, accountRepo.updates, 1)
	require.Contains(t, accountRepo.updates[0], "tls_fingerprint_profile_id")
	_, err := json.Marshal(accountRepo.updates[0])
	require.NoError(t, err)
}
