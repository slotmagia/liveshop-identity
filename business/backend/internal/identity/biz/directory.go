package biz

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/lvtuopen-ai/kernel-go/principal"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
)

// DirectoryRepository exposes aggregate-level reads and transactions. A
// command that changes several Identity facts is one method so callers cannot
// accidentally split its transaction.
type DirectoryRepository interface {
	ResolvePrincipalContext(ctx context.Context, subject string) (model.PrincipalContext, error)
	ValidateActiveSession(ctx context.Context, sessionID, subject string, selected model.SelectedContext, expectedContextVersion uint64) error
	ResolveShopByID(ctx context.Context, shopID int64) (model.ShopResolution, error)
	ListOrganizationSubtree(ctx context.Context, organizationID, rootUnitID int64) ([]int64, uint64, error)
	BatchGetSubjects(ctx context.Context, subjects []string) ([]model.Subject, error)
	ResolveLegacySubjects(ctx context.Context, realm principal.Realm, legacyUIDs []int64) ([]model.Subject, error)
	ListOrganizationDirectory(ctx context.Context, organizationID, merchantID int64) (OrganizationDirectory, error)
	CreateOrganizationUnit(ctx context.Context, command CreateOrganizationUnit) (OrganizationUnitResult, error)
	ProvisionMember(ctx context.Context, command ProvisionMember) (ProvisionMemberResult, error)
	ReplaceMemberAccess(ctx context.Context, command ReplaceMemberAccess) (ProvisionMemberResult, error)
	UpdateMember(ctx context.Context, command UpdateMember) (UpdateMemberResult, error)
}

type Directory struct{ repository DirectoryRepository }

type OrganizationUnit struct {
	ID, ParentID int64
	Name         string
	Status       model.Status
	Version      uint64
}
type MemberDirectoryItem struct {
	Member           model.WorkforceMember
	DisplayName      string
	PrincipalType    principal.Type
	SubjectStatus    model.Status
	SubjectVersion   uint64
	Credential       ManagedCredential
	ShopIDs, UnitIDs []int64
}
type ShopDirectoryItem struct {
	Context    model.ShopContext
	Name, Code string
	Status     model.Status
	Version    uint64
}
type OrganizationDirectory struct {
	Organization model.Organization
	Units        []OrganizationUnit
	Members      []MemberDirectoryItem
	Shops        []ShopDirectoryItem
}

func NewDirectory(repository DirectoryRepository) *Directory {
	return &Directory{repository: repository}
}

func (d *Directory) OrganizationDirectory(ctx context.Context, organizationID, merchantID int64) (OrganizationDirectory, error) {
	if d.repository == nil {
		return OrganizationDirectory{}, model.ErrUnavailable
	}
	if organizationID <= 0 {
		return OrganizationDirectory{}, model.ErrInvalidContext
	}
	return d.repository.ListOrganizationDirectory(ctx, organizationID, merchantID)
}

func (d *Directory) ResolveShopByID(ctx context.Context, shopID int64) (model.ShopResolution, error) {
	if d.repository == nil {
		return model.ShopResolution{}, model.ErrUnavailable
	}
	resolved, err := d.repository.ResolveShopByID(ctx, shopID)
	if err != nil {
		return model.ShopResolution{}, err
	}
	if err := resolved.Validate(); err != nil {
		return model.ShopResolution{}, err
	}
	return resolved, nil
}

func (d *Directory) OrganizationSubtree(ctx context.Context, organizationID, rootUnitID int64) ([]int64, uint64, error) {
	if d.repository == nil {
		return nil, 0, model.ErrUnavailable
	}
	return d.repository.ListOrganizationSubtree(ctx, organizationID, rootUnitID)
}

func (d *Directory) Subjects(ctx context.Context, subjects []string) ([]model.Subject, error) {
	if d.repository == nil {
		return nil, model.ErrUnavailable
	}
	return d.repository.BatchGetSubjects(ctx, subjects)
}

func (d *Directory) LegacySubjects(ctx context.Context, realm principal.Realm, legacyUIDs []int64) ([]model.Subject, error) {
	if d.repository == nil {
		return nil, model.ErrUnavailable
	}
	if !realm.Valid() || len(legacyUIDs) == 0 || len(legacyUIDs) > 200 {
		return nil, model.ErrInvalidContext
	}
	seen := make(map[int64]struct{}, len(legacyUIDs))
	normalized := make([]int64, 0, len(legacyUIDs))
	for _, legacyUID := range legacyUIDs {
		if legacyUID <= 0 {
			return nil, model.ErrInvalidContext
		}
		if _, exists := seen[legacyUID]; exists {
			continue
		}
		seen[legacyUID] = struct{}{}
		normalized = append(normalized, legacyUID)
	}
	return d.repository.ResolveLegacySubjects(ctx, realm, normalized)
}

func (d *Directory) ResolvePrincipalContext(ctx context.Context, subject string, selected model.SelectedContext) (model.PrincipalContext, error) {
	if d.repository == nil {
		return model.PrincipalContext{}, model.ErrUnavailable
	}
	if subject == "" {
		return model.PrincipalContext{}, model.ErrNotFound
	}
	resolved, err := d.repository.ResolvePrincipalContext(ctx, subject)
	if err != nil {
		return model.PrincipalContext{}, err
	}
	if err := resolved.ValidateSelected(selected); err != nil {
		return model.PrincipalContext{}, err
	}
	resolved.Selected = selected
	return resolved, nil
}

func (d *Directory) ValidateSelectedContext(ctx context.Context, subject string, selected model.SelectedContext, identityVersion, accessVersion uint64) (model.PrincipalContext, error) {
	resolved, err := d.ResolvePrincipalContext(ctx, subject, selected)
	if err != nil {
		return model.PrincipalContext{}, err
	}
	if identityVersion != 0 && resolved.Subject.Version != identityVersion {
		return model.PrincipalContext{}, model.ErrConflict
	}
	if accessVersion != 0 && resolved.Member.AccessVersion != accessVersion {
		return model.PrincipalContext{}, model.ErrConflict
	}
	return resolved, nil
}

func (d *Directory) ResolveAuthenticatedPrincipalContext(ctx context.Context, sessionID, subject string, selected model.SelectedContext, expectedContextVersion uint64) (model.PrincipalContext, error) {
	if d.repository == nil || sessionID == "" || expectedContextVersion == 0 {
		return model.PrincipalContext{}, model.ErrInactive
	}
	if err := d.repository.ValidateActiveSession(ctx, sessionID, subject, selected, expectedContextVersion); err != nil {
		return model.PrincipalContext{}, err
	}
	// Guests have no credential-owned shop list. Their selected shop is owned by
	// the durable session row already checked above, so validating it against an
	// empty credential list would incorrectly reject every legitimate guest.
	resolved, err := d.repository.ResolvePrincipalContext(ctx, subject)
	if err != nil {
		return model.PrincipalContext{}, err
	}
	if resolved.Subject.PrincipalType == principal.TypeGuest {
		if !selected.ShopContext.Complete() || selected.OrganizationID != 0 {
			return model.PrincipalContext{}, model.ErrInvalidContext
		}
		resolved.Selected = selected
		return resolved, nil
	}
	return d.ResolvePrincipalContext(ctx, subject, selected)
}

type CreateOrganizationUnit struct {
	IdempotencyKey  string
	OrganizationID  int64
	UnitID          int64
	ParentUnitID    int64
	Name            string
	ExpectedVersion uint64
}

type OrganizationUnitResult struct {
	OrganizationID int64
	UnitID         int64
	Version        uint64
}

func (d *Directory) CreateOrganizationUnit(ctx context.Context, command CreateOrganizationUnit) (OrganizationUnitResult, error) {
	if d.repository == nil {
		return OrganizationUnitResult{}, model.ErrUnavailable
	}
	if command.IdempotencyKey == "" || command.OrganizationID <= 0 || command.UnitID <= 0 ||
		command.Name == "" || command.ExpectedVersion == 0 || command.ParentUnitID == command.UnitID {
		return OrganizationUnitResult{}, model.ErrConflict
	}
	return d.repository.CreateOrganizationUnit(ctx, command)
}

type ProvisionMember struct {
	OperationID          string
	IdempotencyKey       string
	Subject              string
	Realm                principal.Realm
	PrincipalType        principal.Type
	DisplayName          string
	OrganizationID       int64
	MerchantID           int64
	MemberType           model.MemberType
	OrganizationUnitIDs  []int64
	ShopIDs              []int64
	AssignmentKind       model.AssignmentKind
	CredentialKind       string
	CredentialNamespace  string
	NormalizedIdentifier string
	Password             string
	RoleIDs              []int64
	RequestHash          [32]byte
}

type ProvisionMemberResult struct {
	MemberID      int64
	Subject       string
	Status        model.MemberStatus
	AccessVersion uint64
	OperationID   string
}

func (d *Directory) ProvisionMember(ctx context.Context, command ProvisionMember) (ProvisionMemberResult, error) {
	if d.repository == nil {
		return ProvisionMemberResult{}, model.ErrUnavailable
	}
	if err := validateProvision(command); err != nil {
		return ProvisionMemberResult{}, err
	}
	command.RoleIDs = normalizedIDs(command.RoleIDs)
	command.ShopIDs = normalizedIDs(command.ShopIDs)
	command.OrganizationUnitIDs = normalizedIDs(command.OrganizationUnitIDs)
	stable := command
	stable.RequestHash = [32]byte{}
	requestHash, err := CommandHash(stable)
	if err != nil {
		return ProvisionMemberResult{}, err
	}
	command.RequestHash = requestHash
	return d.repository.ProvisionMember(ctx, command)
}

type ReplaceMemberAccess struct {
	OperationID           string
	IdempotencyKey        string
	MemberID              int64
	ExpectedAccessVersion uint64
	OrganizationUnitIDs   []int64
	ShopIDs               []int64
	AssignmentKind        model.AssignmentKind
}

type UpdateMember struct {
	OperationID             string
	IdempotencyKey          string
	Subject                 string
	MerchantID              int64
	DisplayName             string
	MemberType              model.MemberType
	ExpectedIdentityVersion uint64
	ExpectedAccessVersion   uint64
	OrganizationUnitIDs     []int64
	ShopIDs                 []int64
	RoleIDs                 []int64
	AssignmentKind          model.AssignmentKind
	RequestHash             [32]byte
}

type UpdateMemberResult struct {
	MemberID                       int64
	Subject                        string
	Status                         model.MemberStatus
	IdentityVersion, AccessVersion uint64
	OperationID                    string
}

func (d *Directory) UpdateMember(ctx context.Context, command UpdateMember) (UpdateMemberResult, error) {
	if d.repository == nil {
		return UpdateMemberResult{}, model.ErrUnavailable
	}
	command.DisplayName = strings.TrimSpace(command.DisplayName)
	command.RoleIDs = normalizedIDs(command.RoleIDs)
	command.ShopIDs = normalizedIDs(command.ShopIDs)
	command.OrganizationUnitIDs = normalizedIDs(command.OrganizationUnitIDs)
	if command.OperationID == "" || command.IdempotencyKey == "" || command.Subject == "" ||
		command.MerchantID <= 0 || command.DisplayName == "" || command.ExpectedIdentityVersion == 0 ||
		command.ExpectedAccessVersion == 0 || len(command.RoleIDs) == 0 {
		return UpdateMemberResult{}, model.ErrConflict
	}
	if command.MemberType == model.MemberStaff && command.AssignmentKind != model.AssignmentOperate {
		return UpdateMemberResult{}, model.ErrInvalidAssignment
	}
	if command.MemberType == model.MemberAnchor && command.AssignmentKind != model.AssignmentAnchor {
		return UpdateMemberResult{}, model.ErrInvalidAssignment
	}
	if command.MemberType != model.MemberStaff && command.MemberType != model.MemberAnchor {
		return UpdateMemberResult{}, model.ErrConflict
	}
	if !assignmentValid(command.AssignmentKind, command.ShopIDs) || (command.MemberType == model.MemberStaff && len(command.ShopIDs) == 0) {
		return UpdateMemberResult{}, model.ErrInvalidAssignment
	}
	stable := command
	stable.RequestHash = [32]byte{}
	requestHash, err := CommandHash(stable)
	if err != nil {
		return UpdateMemberResult{}, err
	}
	command.RequestHash = requestHash
	return d.repository.UpdateMember(ctx, command)
}

func (d *Directory) ReplaceMemberAccess(ctx context.Context, command ReplaceMemberAccess) (ProvisionMemberResult, error) {
	if d.repository == nil {
		return ProvisionMemberResult{}, model.ErrUnavailable
	}
	if command.OperationID == "" || command.IdempotencyKey == "" || command.MemberID <= 0 ||
		command.ExpectedAccessVersion == 0 || !assignmentValid(command.AssignmentKind, command.ShopIDs) {
		return ProvisionMemberResult{}, model.ErrInvalidAssignment
	}
	return d.repository.ReplaceMemberAccess(ctx, command)
}

func validateProvision(command ProvisionMember) error {
	if command.OperationID == "" || command.IdempotencyKey == "" || command.Subject == "" ||
		command.OrganizationID <= 0 || command.DisplayName == "" || command.CredentialKind == "" ||
		command.CredentialNamespace == "" || command.NormalizedIdentifier == "" || len(command.Password) < 6 || len(command.RoleIDs) == 0 {
		return model.ErrConflict
	}
	if expected := model.ExpectedPrincipalType(command.MemberType); expected == "" ||
		command.PrincipalType != expected || !command.PrincipalType.AllowsRealm(command.Realm) {
		return model.ErrConflict
	}
	if command.MemberType == model.MemberOwner || command.MemberType == model.MemberStaff || command.MemberType == model.MemberAnchor {
		if command.MerchantID <= 0 {
			return model.ErrConflict
		}
	}
	if !assignmentValid(command.AssignmentKind, command.ShopIDs) {
		return model.ErrInvalidAssignment
	}
	if command.MemberType == model.MemberAnchor && command.AssignmentKind != model.AssignmentAnchor {
		return model.ErrInvalidAssignment
	}
	if command.MemberType == model.MemberStaff && command.AssignmentKind != model.AssignmentOperate {
		return model.ErrInvalidAssignment
	}
	return nil
}

func assignmentValid(kind model.AssignmentKind, shops []int64) bool {
	seen := make(map[int64]struct{}, len(shops))
	for _, shopID := range shops {
		if shopID <= 0 {
			return false
		}
		if _, duplicate := seen[shopID]; duplicate {
			return false
		}
		seen[shopID] = struct{}{}
	}
	if kind == model.AssignmentAnchor {
		return len(shops) == 1
	}
	return kind == model.AssignmentOperate
}

func normalizedIDs(values []int64) []int64 {
	seen := map[int64]bool{}
	result := make([]int64, 0, len(values))
	for _, value := range values {
		if value > 0 && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	sort.Slice(result, func(i, j int) bool { return result[i] < result[j] })
	return result
}

// CommandHash is shared by repository implementations to detect a retry that
// reused an idempotency key for different input. It is deterministic because
// command slices are preserved in caller order and IDs are validated unique.
func CommandHash(command any) ([32]byte, error) {
	payload, err := json.Marshal(command)
	if err != nil {
		return [32]byte{}, fmt.Errorf("identity: encode command: %w", err)
	}
	return sha256.Sum256(payload), nil
}
