package merchant_governance

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant_governance/model"
)

// Repository is implemented by the Identity data layer. Intervene is one
// transaction: shop-scope check, optimistic write, command ledger, local
// audit and outbox event.
type Repository interface {
	List(context.Context, model.Query) (model.Page, error)
	Audit(context.Context, model.AuditQuery) ([]model.AuditItem, error)
	Intervene(context.Context, model.InterveneCommand) (model.Capability, bool, error)
}
