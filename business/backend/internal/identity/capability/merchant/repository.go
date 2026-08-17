package merchant

import (
	"context"

	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/merchant/model"
)

type Repository interface {
	ListMerchants(context.Context) ([]model.Merchant, error)
	ListMerchantPage(context.Context, model.Query) (model.Page, error)
	GetMerchant(context.Context, int64) (model.Record, error)
	CreateMerchant(context.Context, model.CreateCommand) (model.CreateResult, bool, error)
	UpdateMerchant(context.Context, model.UpdateCommand) (model.Record, bool, error)
	UpdateProfile(context.Context, model.UpdateProfileCommand) (model.Record, bool, error)
	ResetOwnerPassword(context.Context, model.ResetPasswordCommand) (bool, error)
	CloseMerchant(context.Context, model.CloseCommand) (model.Record, bool, error)
}
