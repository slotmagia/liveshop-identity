// Package customer_service owns customer-service account use cases and repository ports.
package customer_service

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer_service/model"
)

// Repository is implemented by the Identity data layer. Every mutation is a
// single transaction containing the scope check, optimistic write and command
// ledger completion.
type Repository interface {
	List(context.Context, model.Query) (model.Page, error)
	Save(context.Context, model.SaveCommand) (model.Account, bool, error)
	Delete(context.Context, model.DeleteCommand) (model.DeleteResult, bool, error)
}
