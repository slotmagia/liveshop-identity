package logic

import (
	"context"
	"sort"
	"strings"
	"time"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/merch/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

func (l *Logic) AccountSessions(ctx context.Context, query appmodel.AccountSessionQuery) (appmodel.AccountSessionPage, error) {
	claims := authctx.Caller(ctx)
	if claims.Subject == "" || claims.MerchantID <= 0 || claims.OrganizationID <= 0 {
		return appmodel.AccountSessionPage{}, model.ErrInvalidContext
	}
	if l.users == nil {
		return appmodel.AccountSessionPage{}, model.ErrUnavailable
	}
	page, pageSize, err := normalizeAccountSessionQuery(query)
	if err != nil {
		return appmodel.AccountSessionPage{}, err
	}
	values, err := l.users.OwnSessions(ctx, l.userScope(ctx), claims.Subject)
	if err != nil {
		return appmodel.AccountSessionPage{}, err
	}
	now := time.Now().UTC()
	items := make([]appmodel.AccountSession, 0, len(values))
	for _, value := range values {
		if !accountSessionVisible(value, query.Status, now) {
			continue
		}
		items = append(items, appmodel.AccountSession{
			ID: value.ID, DeviceName: value.DeviceName, IPAddress: value.IPAddress, UserAgent: value.UserAgent,
			Status: value.Status, CreatedAt: value.CreatedAt, LastRefreshedAt: value.LastRefreshedAt, ExpiresAt: value.ExpiresAt,
			Current: value.ID == claims.SessionID && value.Status == "ACTIVE",
		})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].Current != items[j].Current {
			return items[i].Current
		}
		return items[i].LastRefreshedAt > items[j].LastRefreshedAt
	})
	total := int64(len(items))
	start := (page - 1) * pageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + pageSize
	if end > len(items) {
		end = len(items)
	}
	return appmodel.AccountSessionPage{Items: items[start:end], Page: page, PageSize: pageSize, Total: total}, nil
}

func (l *Logic) RevokeAccountSession(ctx context.Context, input appmodel.RevokeAccountSession) (appmodel.RevokeAccountSessionResult, error) {
	claims := authctx.Caller(ctx)
	if claims.Subject == "" || claims.SessionID == "" || claims.MerchantID <= 0 || claims.OrganizationID <= 0 {
		return appmodel.RevokeAccountSessionResult{}, model.ErrInvalidContext
	}
	if l.users == nil {
		return appmodel.RevokeAccountSessionResult{}, model.ErrUnavailable
	}
	sessionID := strings.TrimSpace(input.SessionID)
	if sessionID == "" || strings.TrimSpace(input.IdempotencyKey) == "" || strings.TrimSpace(input.OperationID) == "" {
		return appmodel.RevokeAccountSessionResult{}, model.ErrConflict
	}
	if err := l.users.RevokeOwnSessions(ctx, biz.RevokeSessions{
		IdempotencyKey: input.IdempotencyKey, OperationID: input.OperationID,
		Subject: claims.Subject, ActorSubject: claims.Subject, SessionID: sessionID,
		Reason: "SELF_REVOKED", Scope: l.userScope(ctx),
	}); err != nil {
		return appmodel.RevokeAccountSessionResult{}, err
	}
	return appmodel.RevokeAccountSessionResult{CurrentRevoked: sessionID == claims.SessionID}, nil
}

func normalizeAccountSessionQuery(query appmodel.AccountSessionQuery) (int, int, error) {
	status := strings.ToUpper(strings.TrimSpace(query.Status))
	if status != "" && status != "ACTIVE" && status != "REVOKED" {
		return 0, 0, model.ErrConflict
	}
	page := query.Page
	if page <= 0 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize <= 0 || pageSize > 100 {
		pageSize = 20
	}
	return page, pageSize, nil
}

func accountSessionVisible(value biz.ManagedSession, status string, now time.Time) bool {
	expires, err := time.Parse(time.RFC3339Nano, value.ExpiresAt)
	if err != nil {
		expires, err = time.Parse(time.RFC3339, value.ExpiresAt)
	}
	if err != nil || !expires.After(now) {
		return false
	}
	wanted := strings.ToUpper(strings.TrimSpace(status))
	if wanted == "" {
		return true
	}
	return strings.ToUpper(strings.TrimSpace(value.Status)) == wanted
}
