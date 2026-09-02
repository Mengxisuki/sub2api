package service

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/model"
	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

const (
	// defaultModeClaudeCodeBun 使用真实 Claude Code (Bun) ClientHello 的内置模拟模式
	defaultModeClaudeCodeBun = "claude_code_bun"
	// defaultModeNode24 旧版 Node.js 24.x 模拟模式（保留兼容，含 ECH GREASE）
	defaultModeNode24 = "node_24"
)

// builtinProfileSeeds 启动时按名称幂等种入的内置模拟模板，
// 使前端账号下拉与模板管理页可以直接选择这两个模拟模式。
var builtinProfileSeeds = []*model.TLSFingerprintProfile{
	{
		Name:          "Claude Code (Bun)",
		Description:   tlsFingerprintStrPtr("内置模拟模式：复刻真实 Claude Code 2.1.220 (Bun 1.4.0) ClientHello，带 padding、无 ECH GREASE（JA3 d871d02cecbde59abbf8f4806134addf）"),
		ALPNProtocols: []string{"http/1.1"},
		Extensions:    []uint16{0, 23, 65281, 10, 11, 35, 16, 5, 13, 18, 51, 45, 43, 21},
	},
	{
		Name:          "Node.js 24.x (legacy)",
		Description:   tlsFingerprintStrPtr("旧版内置模拟模式：Node.js 24.x ClientHello（固定 ECH GREASE、无 padding，行为特征明显，不建议用于模拟 Claude Code）"),
		ALPNProtocols: []string{"http/1.1"},
		Extensions:    []uint16{0, 65037, 23, 65281, 10, 11, 35, 16, 5, 13, 18, 51, 45, 43},
	},
}

func tlsFingerprintStrPtr(s string) *string {
	return &s
}

// TLSFingerprintProfileRepository 定义 TLS 指纹模板的数据访问接口
type TLSFingerprintProfileRepository interface {
	List(ctx context.Context) ([]*model.TLSFingerprintProfile, error)
	GetByID(ctx context.Context, id int64) (*model.TLSFingerprintProfile, error)
	Create(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error)
	Update(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error)
	Delete(ctx context.Context, id int64) error
}

// TLSFingerprintProfileCache 定义 TLS 指纹模板的缓存接口
type TLSFingerprintProfileCache interface {
	Get(ctx context.Context) ([]*model.TLSFingerprintProfile, bool)
	Set(ctx context.Context, profiles []*model.TLSFingerprintProfile) error
	Invalidate(ctx context.Context) error
	NotifyUpdate(ctx context.Context) error
	SubscribeUpdates(ctx context.Context, handler func())
}

// TLSFingerprintProfileService TLS 指纹模板管理服务
type TLSFingerprintProfileService struct {
	repo  TLSFingerprintProfileRepository
	cache TLSFingerprintProfileCache
	// accountRepo 仅使用 UpdateExtra，将随机选择的模板固化为账号生命周期身份。
	accountRepo TLSFingerprintAccountRepository

	// 本地 ID→Profile 映射缓存，用于 DoWithTLS 热路径快速查找
	localCache map[int64]*model.TLSFingerprintProfile
	localMu    sync.RWMutex

	// defaultMode 账号启用TLS指纹但未绑定模板时的内置默认模拟模式
	defaultMode string
	// randomSelections 是持久化失败/尚未生效时的进程内兜底，避免热路径重抽。
	randomSelections map[int64]int64
	randomMu         sync.RWMutex
}

// TLSFingerprintAccountRepository 是 TLS 身份绑定所需的最小账号仓储契约。
type TLSFingerprintAccountRepository interface {
	UpdateExtra(ctx context.Context, id int64, updates map[string]any) error
}

// NewTLSFingerprintProfileService 创建 TLS 指纹模板服务
func NewTLSFingerprintProfileService(
	repo TLSFingerprintProfileRepository,
	cache TLSFingerprintProfileCache,
	cfg *config.Config,
	accountRepo TLSFingerprintAccountRepository,
) *TLSFingerprintProfileService {
	svc := &TLSFingerprintProfileService{
		repo:             repo,
		cache:            cache,
		accountRepo:      accountRepo,
		localCache:       make(map[int64]*model.TLSFingerprintProfile),
		randomSelections: make(map[int64]int64),
		defaultMode:      defaultModeClaudeCodeBun,
	}
	if cfg != nil && cfg.Gateway.TLSFingerprint.DefaultMode != "" {
		svc.defaultMode = cfg.Gateway.TLSFingerprint.DefaultMode
	}

	ctx := context.Background()
	// 启动时种入内置模拟模板（可配置关闭；失败不阻断启动）
	if cfg == nil || cfg.Gateway.TLSFingerprint.SeedBuiltinProfiles {
		if err := svc.ensureBuiltinProfiles(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to seed built-in profiles: %v", err)
		}
	}
	if err := svc.reloadFromDB(ctx); err != nil {
		logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to load profiles from DB on startup: %v", err)
		if fallbackErr := svc.refreshLocalCache(ctx); fallbackErr != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to load profiles from cache fallback on startup: %v", fallbackErr)
		}
	}

	if cache != nil {
		cache.SubscribeUpdates(ctx, func() {
			if err := svc.refreshLocalCache(context.Background()); err != nil {
				logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to refresh cache on notification: %v", err)
			}
		})
	}

	return svc
}

// ensureBuiltinProfiles 确保内置模拟模板已存在于模板表（按名称幂等）。
func (s *TLSFingerprintProfileService) ensureBuiltinProfiles(ctx context.Context) error {
	existing, err := s.repo.List(ctx)
	if err != nil {
		return err
	}
	existingNames := make(map[string]struct{}, len(existing))
	for _, p := range existing {
		existingNames[p.Name] = struct{}{}
	}

	for _, seed := range builtinProfileSeeds {
		if _, ok := existingNames[seed.Name]; ok {
			continue
		}
		if _, err := s.repo.Create(ctx, seed); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to seed built-in profile %q: %v", seed.Name, err)
		}
	}
	return nil
}

// --- CRUD ---

// List 获取所有模板
func (s *TLSFingerprintProfileService) List(ctx context.Context) ([]*model.TLSFingerprintProfile, error) {
	return s.repo.List(ctx)
}

// GetByID 根据 ID 获取模板
func (s *TLSFingerprintProfileService) GetByID(ctx context.Context, id int64) (*model.TLSFingerprintProfile, error) {
	return s.repo.GetByID(ctx, id)
}

// Create 创建模板
func (s *TLSFingerprintProfileService) Create(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	created, err := s.repo.Create(ctx, profile)
	if err != nil {
		return nil, err
	}

	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)

	return created, nil
}

// Update 更新模板
func (s *TLSFingerprintProfileService) Update(ctx context.Context, profile *model.TLSFingerprintProfile) (*model.TLSFingerprintProfile, error) {
	if err := profile.Validate(); err != nil {
		return nil, err
	}

	updated, err := s.repo.Update(ctx, profile)
	if err != nil {
		return nil, err
	}

	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)

	return updated, nil
}

// Delete 删除模板
func (s *TLSFingerprintProfileService) Delete(ctx context.Context, id int64) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return err
	}

	refreshCtx, cancel := s.newCacheRefreshContext()
	defer cancel()
	s.invalidateAndNotify(refreshCtx)

	return nil
}

// --- 热路径：运行时 Profile 查找 ---

// GetProfileByID 根据 ID 从本地缓存获取 Profile（用于 DoWithTLS 热路径）
// 返回 nil 表示未找到，调用方应 fallback 到内置默认 Profile
func (s *TLSFingerprintProfileService) GetProfileByID(id int64) *tlsfingerprint.Profile {
	s.localMu.RLock()
	p, ok := s.localCache[id]
	s.localMu.RUnlock()

	if ok && p != nil {
		return p.ToTLSProfile()
	}
	return nil
}

// getRandomProfile 从本地缓存中随机选择一个 Profile
func (s *TLSFingerprintProfileService) getRandomProfile() *model.TLSFingerprintProfile {
	s.localMu.RLock()
	defer s.localMu.RUnlock()

	if len(s.localCache) == 0 {
		return nil
	}

	// 收集所有 profile
	profiles := make([]*model.TLSFingerprintProfile, 0, len(s.localCache))
	for _, p := range s.localCache {
		if p != nil {
			profiles = append(profiles, p)
		}
	}
	if len(profiles) == 0 {
		return nil
	}

	return profiles[rand.IntN(len(profiles))]
}

// ResolveTLSProfile 根据 Account 的配置解析出运行时 TLS Profile
//
// 逻辑：
//  1. 未启用 TLS 指纹 → 返回 nil（不伪装）
//  2. 启用 + 绑定了 profile_id → 从缓存查找对应 profile
//  3. 启用 + 未绑定或找不到 → 返回空 Profile（使用代码内置默认值）
func (s *TLSFingerprintProfileService) ResolveTLSProfile(account *Account) *tlsfingerprint.Profile {
	if account == nil || !account.IsTLSFingerprintEnabled() {
		return nil
	}
	id := account.GetTLSFingerprintProfileID()
	if id > 0 {
		if p := s.GetProfileByID(id); p != nil {
			return p
		}
	}
	if id == -1 {
		persistCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		selected, ok := s.resolveStableRandomProfile(persistCtx, account.ID)
		if ok {
			return selected.ToTLSProfile()
		}
	}
	// TLS 启用但无绑定 profile → 使用配置的内置默认模拟模式
	switch s.defaultMode {
	case defaultModeNode24:
		return tlsfingerprint.NodeJS24Profile()
	default:
		return tlsfingerprint.ClaudeCodeBunProfile()
	}
}

func (s *TLSFingerprintProfileService) resolveStableRandomProfile(ctx context.Context, accountID int64) (*model.TLSFingerprintProfile, bool) {
	if accountID <= 0 {
		return nil, false
	}
	s.randomMu.RLock()
	selectedID, ok := s.randomSelections[accountID]
	s.randomMu.RUnlock()
	if ok {
		if profile := s.GetProfileByID(selectedID); profile != nil {
			return s.getProfileModelByID(selectedID), true
		}
		return nil, false
	}

	selected := s.getRandomProfile()
	if selected == nil {
		return nil, false
	}

	s.randomMu.Lock()
	selectedID, alreadySelected := s.randomSelections[accountID]
	if alreadySelected {
		s.randomMu.Unlock()
		return s.getProfileModelByID(selectedID), s.GetProfileByID(selectedID) != nil
	}
	s.randomSelections[accountID] = selected.ID
	s.randomMu.Unlock()

	if s.accountRepo != nil {
		if err := s.accountRepo.UpdateExtra(ctx, accountID, map[string]any{
			"tls_fingerprint_profile_id": selected.ID,
		}); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to persist random profile for account %d: %v", accountID, err)
		}
	}

	return selected, true
}

func (s *TLSFingerprintProfileService) getProfileModelByID(id int64) *model.TLSFingerprintProfile {
	s.localMu.RLock()
	defer s.localMu.RUnlock()
	if profile, ok := s.localCache[id]; ok {
		return profile
	}
	return nil
}

// --- 缓存管理 ---

func (s *TLSFingerprintProfileService) refreshLocalCache(ctx context.Context) error {
	if s.cache != nil {
		if profiles, ok := s.cache.Get(ctx); ok {
			s.setLocalCache(profiles)
			return nil
		}
	}
	return s.reloadFromDB(ctx)
}

func (s *TLSFingerprintProfileService) reloadFromDB(ctx context.Context) error {
	profiles, err := s.repo.List(ctx)
	if err != nil {
		return err
	}

	if s.cache != nil {
		if err := s.cache.Set(ctx, profiles); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to set cache: %v", err)
		}
	}

	s.setLocalCache(profiles)
	return nil
}

func (s *TLSFingerprintProfileService) setLocalCache(profiles []*model.TLSFingerprintProfile) {
	m := make(map[int64]*model.TLSFingerprintProfile, len(profiles))
	for _, p := range profiles {
		m[p.ID] = p
	}

	s.localMu.Lock()
	s.localCache = m
	s.localMu.Unlock()
}

func (s *TLSFingerprintProfileService) newCacheRefreshContext() (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), 3*time.Second)
}

func (s *TLSFingerprintProfileService) invalidateAndNotify(ctx context.Context) {
	if s.cache != nil {
		if err := s.cache.Invalidate(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to invalidate cache: %v", err)
		}
	}

	if err := s.reloadFromDB(ctx); err != nil {
		logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to refresh local cache: %v", err)
		s.localMu.Lock()
		s.localCache = make(map[int64]*model.TLSFingerprintProfile)
		s.localMu.Unlock()
	}

	if s.cache != nil {
		if err := s.cache.NotifyUpdate(ctx); err != nil {
			logger.LegacyPrintf("service.tls_fp_profile", "[TLSFPProfileService] Failed to notify cache update: %v", err)
		}
	}
}
