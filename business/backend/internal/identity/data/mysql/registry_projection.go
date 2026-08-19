package mysql

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	platformv1 "github.com/liveshop-platform/contracts/gen/go/platform/v1"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"google.golang.org/protobuf/proto"
)

func (r *AuthorizationRepository) ReplaceRegistrySnapshot(ctx context.Context, s *platformv1.GetActiveCapabilitySnapshotResponse) error {
	if s == nil || s.RegistryRevision == 0 {
		return fmt.Errorf("%w: empty snapshot or zero revision", model.ErrRegistryProjectionStale)
	}
	canonical, e := proto.MarshalOptions{Deterministic: true}.Marshal(s)
	if e != nil {
		return fmt.Errorf("%w: cannot encode snapshot: %v", model.ErrRegistryProjectionStale, e)
	}
	digest := sha256.Sum256(canonical)
	defined := map[string]bool{}
	assignable := map[string]bool{}
	for _, m := range s.Modules {
		if strings.TrimSpace(m.ModuleId) == "" || strings.TrimSpace(m.ReleaseVersion) == "" {
			return fmt.Errorf("%w: module identity is incomplete", model.ErrRegistryProjectionStale)
		}
		for _, p := range m.Permissions {
			if p == nil || p.Code == "" || p.Resource == "" || p.Action == "" || defined[p.Code] {
				code := "<nil>"
				if p != nil {
					code = p.Code
				}
				return fmt.Errorf("%w: module %q has an invalid or duplicate permission %q", model.ErrRegistryProjectionStale, m.ModuleId, code)
			}
			defined[p.Code] = true
		}
	}
	for _, m := range s.Modules {
		for _, c := range m.Contributions {
			// An empty permission list is the explicit Registry contract for a
			// guest-visible storefront contribution. All control surfaces remain
			// permission-gated and fail closed.
			if !completeContributionIdentity(c) {
				id := "<nil>"
				if c != nil {
					id = c.ContributionId
				}
				return fmt.Errorf("%w: module %q has an incomplete contribution %q", model.ErrRegistryProjectionStale, m.ModuleId, id)
			}
			for _, code := range c.RequiredPermissions {
				if !defined[code] {
					return fmt.Errorf("%w: contribution %q references undefined permission %q", model.ErrRegistryProjectionStale, c.ContributionId, code)
				}
				assignable[code] = true
			}
			for _, route := range c.AllowedRoutes {
				if !completeAllowedRoute(c.Surface, route) {
					return fmt.Errorf("%w: contribution %q has an incomplete allowed route", model.ErrRegistryProjectionStale, c.ContributionId)
				}
				for _, code := range route.RequiredPermissions {
					if !defined[code] {
						return fmt.Errorf("%w: contribution %q route %q references undefined permission %q", model.ErrRegistryProjectionStale, c.ContributionId, route.Prefix, code)
					}
					assignable[code] = true
				}
			}
			for _, action := range c.FrontendActions {
				for _, code := range action.RequiredPermissions {
					if !defined[code] {
						return fmt.Errorf("%w: contribution %q action %q references undefined permission %q", model.ErrRegistryProjectionStale, c.ContributionId, action.ActionId, code)
					}
					assignable[code] = true
				}
			}
		}
		for _, operation := range m.HttpOperations {
			if operation.Authentication != "module-session" {
				continue
			}
			for _, code := range operation.RequiredPermissions {
				if !defined[code] {
					return fmt.Errorf("%w: module %q operation %q references undefined permission %q", model.ErrRegistryProjectionStale, m.ModuleId, operation.OperationId, code)
				}
				assignable[code] = true
			}
		}
	}
	tx, e := r.db.BeginTx(ctx, nil)
	if e != nil {
		return e
	}
	defer tx.Rollback()
	var current uint64
	var currentDigest []byte
	e = tx.QueryRowContext(ctx, `SELECT registry_revision,snapshot_digest FROM identity_registry_projection_state WHERE singleton_id=1 FOR UPDATE`).Scan(&current, &currentDigest)
	if e != nil && !errors.Is(e, sql.ErrNoRows) {
		return e
	}
	if current > s.RegistryRevision {
		return model.ErrAuthorizationConflict
	}
	if current == s.RegistryRevision {
		if !equalDigest(currentDigest, digest[:]) {
			return model.ErrAuthorizationConflict
		}
		if _, e = tx.ExecContext(ctx, `UPDATE identity_registry_projection_state SET projected_at=CURRENT_TIMESTAMP(3) WHERE singleton_id=1`); e != nil {
			return e
		}
		return tx.Commit()
	}
	if _, e = tx.ExecContext(ctx, `UPDATE identity_permission_projection SET active=0`); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE identity_contribution_projection SET active=0`); e != nil {
		return e
	}
	if _, e = tx.ExecContext(ctx, `UPDATE identity_module_projection SET active=0`); e != nil {
		return e
	}
	for _, m := range s.Modules {
		for _, p := range m.Permissions {
			if !assignable[p.Code] {
				continue
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO identity_permission_projection(permission_code,module_id,name,resource_code,action,description,registry_revision,active) VALUES(?,?,?,?,?,?,?,1) ON DUPLICATE KEY UPDATE module_id=VALUES(module_id),name=VALUES(name),resource_code=VALUES(resource_code),action=VALUES(action),description=VALUES(description),registry_revision=VALUES(registry_revision),active=1`, p.Code, m.ModuleId, p.Name, p.Resource, p.Action, p.Description, s.RegistryRevision); e != nil {
				return e
			}
		}
		for _, c := range m.Contributions {
			routes := []model.RegistryAllowedRoute{}
			for _, x := range c.AllowedRoutes {
				routes = append(routes, model.RegistryAllowedRoute{Methods: x.Methods, Prefix: x.Prefix, RequiredPermissions: x.RequiredPermissions})
			}
			capability := contributionModel(s.RegistryRevision, m, c, routes)
			raw, _ := json.Marshal(capability)
			if _, e = tx.ExecContext(ctx, `INSERT INTO identity_contribution_projection(module_id,module_version,contribution_id,surface,capability_json,registry_revision,active) VALUES(?,?,?,?,?,?,1) ON DUPLICATE KEY UPDATE surface=VALUES(surface),capability_json=VALUES(capability_json),registry_revision=VALUES(registry_revision),active=1`, m.ModuleId, m.ReleaseVersion, c.ContributionId, c.Surface, raw, s.RegistryRevision); e != nil {
				return e
			}
		}
		release, e := catalogRelease(m)
		if e != nil {
			return fmt.Errorf("%w: module %q cannot be projected into the catalog: %v", model.ErrRegistryProjectionStale, m.ModuleId, e)
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO identity_module_projection(module_id,module_version,module_name,release_json,registry_revision,active) VALUES(?,?,?,?,?,1) ON DUPLICATE KEY UPDATE module_name=VALUES(module_name),release_json=VALUES(release_json),registry_revision=VALUES(registry_revision),active=1`, m.ModuleId, m.ReleaseVersion, m.ModuleName, release, s.RegistryRevision); e != nil {
			return e
		}
	}
	_, e = tx.ExecContext(ctx, `INSERT INTO identity_registry_projection_state(singleton_id,registry_revision,snapshot_digest,projected_at) VALUES(1,?,?,CURRENT_TIMESTAMP(3)) ON DUPLICATE KEY UPDATE registry_revision=VALUES(registry_revision),snapshot_digest=VALUES(snapshot_digest),projected_at=VALUES(projected_at)`, s.RegistryRevision, digest[:])
	if e != nil {
		return e
	}
	return tx.Commit()
}

func completeContributionIdentity(c *platformv1.ActiveContribution) bool {
	return c != nil && c.ContributionId != "" && c.Surface != "" && (c.Surface == "shop" || len(c.RequiredPermissions) > 0)
}

func completeAllowedRoute(surface string, route *platformv1.AllowedRoute) bool {
	return route != nil && route.Prefix != "" && len(route.Methods) > 0 && (surface == "shop" || len(route.RequiredPermissions) > 0)
}

func equalDigest(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	var different byte
	for i := range a {
		different |= a[i] ^ b[i]
	}
	return different == 0
}

func contributionModel(revision uint64, m *platformv1.ActiveModuleCapability, c *platformv1.ActiveContribution, routes []model.RegistryAllowedRoute) model.RegistryContribution {
	result := model.RegistryContribution{RegistryRevision: revision, ModuleID: m.ModuleId, ModuleVersion: m.ReleaseVersion, ContributionID: c.ContributionId, Surface: c.Surface, Kind: c.Kind, Route: c.Route, Outlet: c.Outlet, Title: c.Title, Description: c.Description, Icon: c.Icon, Sort: c.Sort, RequiredPermissions: c.RequiredPermissions, AllowedRoutes: routes}
	if c.Navigation != nil {
		result.Navigation = &model.RegistryNavigation{GroupID: c.Navigation.GroupId, GroupTitle: c.Navigation.GroupTitle, GroupSort: c.Navigation.GroupSort}
	}
	if c.Artifact != nil {
		result.Artifact = model.RegistryArtifact{Type: c.Artifact.Type, Name: c.Artifact.Name, Version: c.Artifact.Version, Entry: c.Artifact.Entry, ExportName: c.Artifact.ExportName, Integrity: c.Artifact.Integrity}
	}
	if c.Frontend != nil {
		result.Frontend.Component = c.Frontend.Component
		result.Frontend.Props, _ = json.Marshal(manifestFields(c.Frontend.Props))
		result.Frontend.Events, _ = json.Marshal(manifestEvents(c.Frontend.Events))
		for _, a := range c.Frontend.Actions {
			parameters, _ := json.Marshal(manifestFields(a.Parameters))
			result.Frontend.Actions = append(result.Frontend.Actions, model.RegistryFrontendAction{ID: a.ActionId, Label: a.Label, Description: a.Description, Invocation: a.Invocation, Target: a.Target, Parameters: parameters, RequiredPermissions: a.RequiredPermissions})
		}
	}
	return result
}

func catalogRelease(m *platformv1.ActiveModuleCapability) ([]byte, error) {
	backend := map[string]any{"service": m.GetBackend().GetService(), "origin": m.GetBackend().GetOrigin()}
	type routeGroup struct {
		Surface    string
		Prefix     string
		Operations []any
	}
	grouped := map[string]*routeGroup{}
	for index, operation := range m.HttpOperations {
		if operation == nil || operation.OperationId == "" || operation.Surface == "" || operation.Path == "" {
			if operation == nil {
				return nil, fmt.Errorf("%w: HTTP operation %d is nil", model.ErrRegistryProjectionStale, index)
			}
			return nil, fmt.Errorf("%w: HTTP operation %d is incomplete (id=%q surface=%q path=%q)", model.ErrRegistryProjectionStale, index, operation.OperationId, operation.Surface, operation.Path)
		}
		prefix := operationRoutePrefix(operation.Surface, m.ModuleId, operation.Path)
		key := operation.Surface + "\x00" + prefix
		group := grouped[key]
		if group == nil {
			group = &routeGroup{Surface: operation.Surface, Prefix: prefix}
			grouped[key] = group
		}
		group.Operations = append(group.Operations, manifestHTTPOperation(operation))
	}
	httpRoutes := []any{}
	groupKeys := make([]string, 0, len(grouped))
	for key := range grouped {
		groupKeys = append(groupKeys, key)
	}
	sort.Strings(groupKeys)
	for _, key := range groupKeys {
		group := grouped[key]
		httpRoutes = append(httpRoutes, map[string]any{"surface": group.Surface, "prefix": group.Prefix, "operations": group.Operations})
	}
	backend["httpRoutes"] = httpRoutes
	if m.Grpc != nil {
		backend["grpc"] = manifestGRPC(m.Grpc)
	}
	permissions := []any{}
	for _, permission := range m.Permissions {
		permissions = append(permissions, map[string]any{"code": permission.Code, "name": permission.Name, "resource": permission.Resource, "action": permission.Action, "description": permission.Description})
	}
	contributions := []any{}
	for index, contribution := range m.Contributions {
		if contribution == nil || contribution.ContributionId == "" {
			return nil, fmt.Errorf("%w: contribution %d has no identity", model.ErrRegistryProjectionStale, index)
		}
		contributions = append(contributions, manifestContribution(contribution))
	}
	return json.Marshal(map[string]any{"version": m.ReleaseVersion, "digest": m.ReleaseDigest, "active": true, "backend": backend, "permissions": permissions, "contributions": contributions})
}

func operationRoutePrefix(surface, moduleID, path string) string {
	expected := "/" + surface + "/" + moduleID
	if path == expected || strings.HasPrefix(path, expected+"/") {
		return expected
	}
	// Public and other explicitly declared route groups do not necessarily use
	// the surface/module namespace. The active snapshot currently flattens HTTP
	// operations, so recover their stable two-segment route group from the
	// operation path instead of rejecting a valid registered capability.
	segments := strings.Split(strings.Trim(path, "/"), "/")
	if len(segments) >= 2 {
		return "/" + segments[0] + "/" + segments[1]
	}
	return path
}

func manifestHTTPOperation(operation *platformv1.HttpOperation) map[string]any {
	responses := make([]any, 0, len(operation.Responses))
	for _, response := range operation.Responses {
		responses = append(responses, map[string]any{"status": response.Status, "description": response.Description, "fields": manifestFields(response.Fields)})
	}
	return map[string]any{"id": operation.OperationId, "method": operation.Method, "path": operation.Path, "summary": operation.Summary, "description": operation.Description, "authentication": operation.Authentication, "idempotency": operation.Idempotency, "requiredPermissions": operation.RequiredPermissions, "requestFields": manifestFields(operation.RequestFields), "responses": responses}
}

func manifestFields(fields []*platformv1.CapabilityField) []any {
	result := make([]any, 0, len(fields))
	for _, field := range fields {
		if field == nil {
			continue
		}
		result = append(result, map[string]any{"name": field.Name, "location": field.Location, "type": field.Type, "format": field.Format, "required": field.Required, "description": field.Description, "example": field.Example})
	}
	return result
}

func manifestEvents(events []*platformv1.FrontendEvent) []any {
	result := make([]any, 0, len(events))
	for _, event := range events {
		if event != nil {
			result = append(result, map[string]any{"name": event.Name, "description": event.Description, "payload": manifestFields(event.Payload)})
		}
	}
	return result
}

func manifestAction(action *platformv1.FrontendAction) map[string]any {
	return map[string]any{"id": action.ActionId, "label": action.Label, "description": action.Description, "invocation": action.Invocation, "target": action.Target, "parameters": manifestFields(action.Parameters), "requiredPermissions": action.RequiredPermissions}
}

func manifestContribution(contribution *platformv1.ActiveContribution) map[string]any {
	allowedRoutes := make([]any, 0, len(contribution.AllowedRoutes))
	for _, route := range contribution.AllowedRoutes {
		allowedRoutes = append(allowedRoutes, map[string]any{"methods": route.Methods, "prefix": route.Prefix, "requiredPermissions": route.RequiredPermissions})
	}
	actions := []any{}
	frontend := map[string]any{"component": "", "props": []any{}, "events": []any{}, "actions": actions}
	if contribution.Frontend != nil {
		for _, action := range contribution.Frontend.Actions {
			actions = append(actions, manifestAction(action))
		}
		frontend = map[string]any{"component": contribution.Frontend.Component, "props": manifestFields(contribution.Frontend.Props), "events": manifestEvents(contribution.Frontend.Events), "actions": actions}
	}
	artifact := map[string]any{}
	if contribution.Artifact != nil {
		artifact = map[string]any{"type": contribution.Artifact.Type, "name": contribution.Artifact.Name, "version": contribution.Artifact.Version, "entry": contribution.Artifact.Entry, "exportName": contribution.Artifact.ExportName, "integrity": contribution.Artifact.Integrity}
	}
	value := map[string]any{"id": contribution.ContributionId, "surface": contribution.Surface, "kind": contribution.Kind, "route": contribution.Route, "outlet": contribution.Outlet, "title": contribution.Title, "description": contribution.Description, "icon": contribution.Icon, "sort": contribution.Sort, "requiredPermissions": contribution.RequiredPermissions, "allowedRoutes": allowedRoutes, "artifact": artifact, "frontend": frontend}
	if contribution.Navigation != nil {
		value["navigation"] = map[string]any{"groupId": contribution.Navigation.GroupId, "groupTitle": contribution.Navigation.GroupTitle, "groupSort": contribution.Navigation.GroupSort}
	}
	return value
}

func manifestGRPC(contract *platformv1.GrpcContract) map[string]any {
	methods := make([]any, 0, len(contract.Methods))
	for _, method := range contract.Methods {
		methods = append(methods, map[string]any{"name": method.Name, "fullMethod": method.FullMethod, "summary": method.Summary, "description": method.Description, "invocation": method.Invocation, "idempotency": method.Idempotency, "recommendedDeadlineMs": method.RecommendedDeadlineMs, "requiredPermissions": method.RequiredPermissions, "requestFields": manifestFields(method.RequestFields), "responseFields": manifestFields(method.ResponseFields)})
	}
	return map[string]any{"service": contract.Service, "contractVersion": contract.ContractVersion, "endpoint": contract.Endpoint, "transportSecurity": contract.TransportSecurity, "methods": methods}
}
func (r *AuthorizationRepository) LocalRegistryRevision(ctx context.Context) (uint64, error) {
	var revision uint64
	err := r.db.QueryRowContext(ctx, `SELECT registry_revision FROM identity_registry_projection_state WHERE singleton_id=1`).Scan(&revision)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, nil
	}
	return revision, err
}

func (r *AuthorizationRepository) RegistryReady(ctx context.Context, maxAge time.Duration) error {
	var revision uint64
	var at time.Time
	if e := r.db.QueryRowContext(ctx, `SELECT registry_revision,projected_at FROM identity_registry_projection_state WHERE singleton_id=1`).Scan(&revision, &at); e != nil || revision == 0 || time.Since(at) > maxAge {
		return model.ErrRegistryProjectionStale
	}
	return nil
}
