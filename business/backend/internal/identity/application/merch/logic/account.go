package logic

import (
	"context"
	"errors"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	subscriptionmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/subscription/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

func (l *Logic) Account(ctx context.Context) (appmodel.Account, error) {
	claims := authctx.Caller(ctx)
	if claims.Subject == "" || claims.MerchantID <= 0 || claims.OrganizationID <= 0 {
		return appmodel.Account{}, model.ErrInvalidContext
	}
	if l.directory == nil {
		return appmodel.Account{}, model.ErrUnavailable
	}
	principalContext, err := l.directory.ResolvePrincipalContext(ctx, claims.Subject, model.SelectedContext{
		OrganizationID: claims.OrganizationID,
		ShopContext:    model.ShopContext{MerchantID: claims.MerchantID, ShopID: claims.ShopID},
	})
	if err != nil {
		return appmodel.Account{}, err
	}
	if principalContext.Organization.MerchantID != claims.MerchantID {
		return appmodel.Account{}, model.ErrInvalidContext
	}
	directory, err := l.Directory(ctx)
	if err != nil {
		return appmodel.Account{}, err
	}
	account := appmodel.Account{
		Subject:       principalContext.Subject.ID,
		DisplayName:   principalContext.Subject.DisplayName,
		PrincipalType: claims.PrincipalType.String(),
		Owner:         claims.PrincipalType == principal.TypeMerchantOwner,
		Status:        string(principalContext.Subject.Status),
		Merchant: appmodel.AccountMerchant{
			MerchantID: principalContext.Organization.MerchantID,
			Name:       principalContext.Organization.Name,
			Status:     string(principalContext.Organization.Status),
		},
		CurrentShopID:   claims.ShopID,
		Shops:           accessibleShops(directory.Shops, principalContext, claims.PrincipalType),
		PermissionNames: []string{},
		Organization: appmodel.AccountOrganization{
			ID: directory.Organization.ID, Name: directory.Organization.Name,
			Status: string(directory.Organization.Status), UnitCount: len(directory.Units),
			MemberCount: countableMembers(directory.Members), ShopCount: len(directory.Shops),
		},
	}
	for _, member := range directory.Members {
		if member.Subject == claims.Subject {
			account.Account = member.Credential.Identifier
			if account.DisplayName == "" {
				account.DisplayName = member.DisplayName
			}
			if account.Status == "" {
				account.Status = member.SubjectStatus
			}
			break
		}
	}
	subscription, err := l.accountSubscription(ctx)
	if err != nil {
		return appmodel.Account{}, err
	}
	account.Subscription = subscription
	permissions, err := l.accountPermissions(ctx, principalContext)
	if err != nil {
		return appmodel.Account{}, err
	}
	account.PermissionNames = permissions
	return account, nil
}

func (l *Logic) AccountSecurity(ctx context.Context) (appmodel.AccountSecurity, error) {
	claims := authctx.Caller(ctx)
	if claims.Subject == "" || claims.MerchantID <= 0 || claims.OrganizationID <= 0 {
		return appmodel.AccountSecurity{}, model.ErrInvalidContext
	}
	if l.users == nil {
		return appmodel.AccountSecurity{}, model.ErrUnavailable
	}
	value, err := l.users.OwnAccount(ctx, l.userScope(ctx), claims.Subject)
	if err != nil {
		return appmodel.AccountSecurity{}, err
	}
	return appmodel.AccountSecurity{
		Subject: value.Subject.ID, DisplayName: value.Subject.DisplayName, Account: value.Credential.Identifier,
		PrincipalType: value.Subject.PrincipalType.String(), Owner: value.Member.Type == model.MemberOwner,
		Status: string(value.Subject.Status), ActiveSessions: value.ActiveSessions,
		Credential: appmodel.ManagedCredential{ID: value.Credential.ID, Version: value.Credential.Version, Kind: value.Credential.Kind, Identifier: value.Credential.Identifier, Status: string(value.Credential.Status)},
	}, nil
}

func (l *Logic) ChangeOwnCredential(ctx context.Context, input appmodel.ChangeOwnCredential) (appmodel.ChangeOwnCredentialResult, error) {
	claims := authctx.Caller(ctx)
	if claims.Subject == "" || claims.MerchantID <= 0 || claims.OrganizationID <= 0 || claims.SessionID == "" {
		return appmodel.ChangeOwnCredentialResult{}, model.ErrInvalidContext
	}
	if l.users == nil {
		return appmodel.ChangeOwnCredentialResult{}, model.ErrUnavailable
	}
	value, err := l.users.ChangeOwnCredential(ctx, biz.ChangeOwnCredential{
		IdempotencyKey: input.CommandKey, OperationID: input.CommandKey, Subject: claims.Subject, ActorSubject: claims.Subject,
		SessionID: claims.SessionID, OldPassword: input.OldPassword, Password: input.Password, Scope: l.userScope(ctx),
		ExpectedCredentialVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.ChangeOwnCredentialResult{}, err
	}
	return appmodel.ChangeOwnCredentialResult{
		Credential:      appmodel.ManagedCredential{ID: value.Credential.ID, Version: value.Credential.Version, Kind: value.Credential.Kind, Identifier: value.Credential.Identifier, Status: string(value.Credential.Status)},
		RevokedSessions: value.RevokedSessions, CurrentRetained: value.CurrentRetained, Replayed: value.Replayed,
	}, nil
}

func (l *Logic) accountSubscription(ctx context.Context) (appmodel.SubscriptionCurrent, error) {
	if l.assignments == nil {
		return appmodel.SubscriptionCurrent{PermissionNames: []string{}}, nil
	}
	value, err := l.Subscription(ctx)
	if err == nil {
		if value.PermissionNames == nil {
			value.PermissionNames = []string{}
		}
		return value, nil
	}
	if errors.Is(err, subscriptionmodel.ErrAssignmentNotFound) || errors.Is(err, subscriptionmodel.ErrAssignmentInvalid) {
		return appmodel.SubscriptionCurrent{PermissionNames: []string{}}, nil
	}
	return appmodel.SubscriptionCurrent{}, err
}

func (l *Logic) accountPermissions(ctx context.Context, principalContext model.PrincipalContext) ([]string, error) {
	if l.authorization == nil || principalContext.Member.AccessVersion == 0 {
		return []string{}, nil
	}
	authorization, err := l.authorization.Effective(ctx, l.domain(ctx), principalContext)
	if err != nil {
		if errors.Is(err, model.ErrAuthorizationInvalid) || errors.Is(err, model.ErrUnavailable) {
			return []string{}, nil
		}
		return nil, err
	}
	names, err := l.permissionNameIndex(ctx)
	if err != nil {
		return nil, err
	}
	return permissionLabels(authorization.Permissions, names), nil
}

func accessibleShops(shops []appmodel.Shop, principalContext model.PrincipalContext, principalType principal.Type) []appmodel.Shop {
	out := make([]appmodel.Shop, 0, len(shops))
	if principalType == principal.TypeMerchantOwner && len(principalContext.AvailableShops) == 0 {
		return append(out, shops...)
	}
	allowed := make(map[int64]struct{}, len(principalContext.AvailableShops))
	for _, shop := range principalContext.AvailableShops {
		allowed[shop.ShopID] = struct{}{}
	}
	for _, shop := range shops {
		if _, ok := allowed[shop.ID]; ok {
			out = append(out, shop)
		}
	}
	return out
}

func countableMembers(members []appmodel.Member) int {
	count := 0
	for _, member := range members {
		if member.Type == string(model.MemberStaff) || member.Type == string(model.MemberAnchor) {
			count++
		}
	}
	return count
}
