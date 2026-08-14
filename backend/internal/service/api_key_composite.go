package service

import (
	"context"
	"regexp"
	"sort"
	"strings"
	"time"
)

const (
	// MaxCompositeAPIKeyGroups 限制认证缓存和模型列表聚合的最大规模。
	MaxCompositeAPIKeyGroups = 20
	MaxCompositeKeyPrefixLen = 32
)

var compositeKeyPrefixPattern = regexp.MustCompile(`^[A-Za-z0-9_-]+$`)

// APIKeyCompositeGroupInput 是创建和更新复合映射时的公共输入。
type APIKeyCompositeGroupInput struct {
	GroupID int64  `json:"group_id"`
	Prefix  string `json:"prefix"`
}

// NormalizeCompositeKeyPrefix 规范化并校验用户输入的分组前缀。
func NormalizeCompositeKeyPrefix(prefix string) (display string, normalized string, ok bool) {
	display = strings.TrimSpace(prefix)
	if display == "" || len(display) > MaxCompositeKeyPrefixLen || !compositeKeyPrefixPattern.MatchString(display) {
		return "", "", false
	}
	return display, strings.ToLower(display), true
}

// SplitCompositeModel 按第一个斜杠拆分复合模型，真实模型 ID 可以继续包含斜杠。
func SplitCompositeModel(model string) (prefix string, modelID string, ok bool) {
	model = strings.TrimSpace(model)
	parts := strings.SplitN(model, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	prefix = strings.TrimSpace(parts[0])
	modelID = strings.TrimSpace(parts[1])
	if prefix == "" || modelID == "" {
		return "", "", false
	}
	return prefix, modelID, true
}

// ResolveCompositeModel 根据大小写不敏感前缀选择分组并返回真实模型 ID。
func (k *APIKey) ResolveCompositeModel(model string) (*APIKeyCompositeGroup, string, error) {
	if k == nil || !k.IsComposite {
		return nil, model, nil
	}
	prefix, modelID, ok := SplitCompositeModel(model)
	if !ok {
		return nil, "", ErrCompositeKeyPrefixRequired
	}
	_, normalized, valid := NormalizeCompositeKeyPrefix(prefix)
	if !valid {
		return nil, "", ErrCompositeKeyPrefixInvalid
	}
	for i := range k.CompositeGroups {
		binding := &k.CompositeGroups[i]
		if binding.NormalizedPrefix == normalized {
			return binding, modelID, nil
		}
	}
	return nil, "", ErrCompositeKeyPrefixNotFound
}

// validateCompositeGroupInputs 只处理与数据库无关的结构校验并保留用户顺序。
func validateCompositeGroupInputs(inputs []APIKeyCompositeGroupInput) ([]APIKeyCompositeGroup, error) {
	if len(inputs) == 0 {
		return nil, ErrCompositeKeyGroupsRequired
	}
	if len(inputs) > MaxCompositeAPIKeyGroups {
		return nil, ErrCompositeKeyTooManyGroups
	}
	groups := make([]APIKeyCompositeGroup, 0, len(inputs))
	seenGroups := make(map[int64]struct{}, len(inputs))
	seenPrefixes := make(map[string]struct{}, len(inputs))
	for index, input := range inputs {
		if input.GroupID <= 0 {
			return nil, ErrGroupNotAllowed
		}
		if _, exists := seenGroups[input.GroupID]; exists {
			return nil, ErrCompositeKeyGroupDuplicate
		}
		display, normalized, ok := NormalizeCompositeKeyPrefix(input.Prefix)
		if !ok {
			return nil, ErrCompositeKeyPrefixInvalid
		}
		if _, exists := seenPrefixes[normalized]; exists {
			return nil, ErrCompositeKeyPrefixDuplicate
		}
		seenGroups[input.GroupID] = struct{}{}
		seenPrefixes[normalized] = struct{}{}
		groups = append(groups, APIKeyCompositeGroup{
			GroupID:          input.GroupID,
			Prefix:           display,
			NormalizedPrefix: normalized,
			SortOrder:        index,
		})
	}
	return groups, nil
}

// prepareCompositeGroups 校验分组权限，并为本次新增的数据共享分组保存确认快照。
func (s *APIKeyService) prepareCompositeGroups(
	ctx context.Context,
	user *User,
	inputs []APIKeyCompositeGroupInput,
	existing []APIKeyCompositeGroup,
	confirmed bool,
	noticeVersion int,
) ([]APIKeyCompositeGroup, error) {
	bindings, err := validateCompositeGroupInputs(inputs)
	if err != nil {
		return nil, err
	}
	existingByGroup := make(map[int64]APIKeyCompositeGroup, len(existing))
	for _, binding := range existing {
		existingByGroup[binding.GroupID] = binding
	}
	for i := range bindings {
		binding := &bindings[i]
		group, err := s.groupRepo.GetByID(ctx, binding.GroupID)
		if err != nil {
			return nil, err
		}
		if !s.canUserBindGroup(ctx, user, group) {
			return nil, ErrGroupNotAllowed
		}
		binding.Group = group
		if previous, exists := existingByGroup[binding.GroupID]; exists {
			binding.ID = previous.ID
			binding.APIKeyID = previous.APIKeyID
			binding.DataSharingNoticeVersion = previous.DataSharingNoticeVersion
			binding.DataSharingConfirmedAt = previous.DataSharingConfirmedAt
			continue
		}
		if group.DataSharingEnabled {
			version, confirmedAt, err := s.validateCurrentDataSharingConsent(ctx, group, confirmed, noticeVersion)
			if err != nil {
				return nil, err
			}
			binding.DataSharingNoticeVersion = version
			binding.DataSharingConfirmedAt = &confirmedAt
		}
	}
	sort.SliceStable(bindings, func(i, j int) bool { return bindings[i].SortOrder < bindings[j].SortOrder })
	return bindings, nil
}

// cloneCompositeBindings 避免认证请求对缓存快照中的切片原地修改。
func cloneCompositeBindings(bindings []APIKeyCompositeGroup) []APIKeyCompositeGroup {
	if len(bindings) == 0 {
		return nil
	}
	out := make([]APIKeyCompositeGroup, len(bindings))
	copy(out, bindings)
	for i := range out {
		if bindings[i].DataSharingConfirmedAt != nil {
			value := *bindings[i].DataSharingConfirmedAt
			out[i].DataSharingConfirmedAt = &value
		}
	}
	return out
}

// compositeConsentSnapshot 返回兼容旧 API 字段的最近一次确认信息。
func compositeConsentSnapshot(bindings []APIKeyCompositeGroup) (int, *int64, *time.Time) {
	for i := len(bindings) - 1; i >= 0; i-- {
		binding := bindings[i]
		if binding.DataSharingNoticeVersion > 0 && binding.DataSharingConfirmedAt != nil {
			groupID := binding.GroupID
			confirmedAt := *binding.DataSharingConfirmedAt
			return binding.DataSharingNoticeVersion, &groupID, &confirmedAt
		}
	}
	return 0, nil, nil
}
