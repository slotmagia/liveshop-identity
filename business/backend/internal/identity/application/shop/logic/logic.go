package logic

import (
	"context"
	"time"

	"github.com/lvtuopen-ai/kernel-go/principal"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/admin/compose"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/biz/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth"
	authmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer"
	customermodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/customer/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment"
	fulfillmentmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/fulfillment/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop"
	shopmodel "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/shop/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/authctx"
)

type Logic struct {
	health     *biz.Health
	otp        *auth.OTP
	customer   *customer.Book
	shops      *shop.Directory
	aftersales *fulfillment.Aftersales
	grants     compose.Grants
}

var _ service.Shop = (*Logic)(nil)

func New(health *biz.Health, otp *auth.OTP, book *customer.Book, shops *shop.Directory, aftersales *fulfillment.Aftersales, grants compose.Grants) *Logic {
	return &Logic{health: health, otp: otp, customer: book, shops: shops, aftersales: aftersales, grants: grants}
}

func (l *Logic) Health(ctx context.Context) (appmodel.Health, error) {
	if l.health == nil {
		return appmodel.Health{}, model.ErrUnavailable
	}
	current, err := l.health.Check(ctx)
	if err != nil {
		return appmodel.Health{}, err
	}
	return appmodel.Health{Status: current.Status}, nil
}

func (l *Logic) CreateLoginOTP(ctx context.Context, input appmodel.CreateLoginOTP) (appmodel.LoginOTP, error) {
	if l.otp == nil {
		return appmodel.LoginOTP{}, authmodel.ErrUnavailable
	}
	challenge, err := l.otp.Request(ctx, authmodel.RequestCommand{ShopCode: input.ShopCode, Phone: input.Phone, Email: input.Email})
	if err != nil {
		return appmodel.LoginOTP{}, err
	}
	return appmodel.LoginOTP{ChallengeID: challenge.ID, TTLSeconds: challenge.TTLSeconds, ExpiresAt: challenge.ExpiresAt.UTC().Format(time.RFC3339Nano)}, nil
}

func (l *Logic) CreateLogin(ctx context.Context, input appmodel.CreateLogin) (appmodel.Login, error) {
	if l.otp == nil {
		return appmodel.Login{}, authmodel.ErrUnavailable
	}
	challenge, err := l.otp.Verify(ctx, authmodel.VerifyCommand{ShopCode: input.ShopCode, ChallengeID: input.ChallengeID, Code: input.Code})
	if err != nil {
		return appmodel.Login{}, err
	}
	return appmodel.Login{ChallengeID: challenge.ID, Verified: true}, nil
}

func (l *Logic) Addresses(ctx context.Context) ([]appmodel.Address, error) {
	if l.customer == nil {
		return nil, customermodel.ErrUnavailable
	}
	items, err := l.customer.Addresses(ctx, customerTenant(ctx), authctx.Caller(ctx).Subject)
	if err != nil {
		return nil, err
	}
	result := make([]appmodel.Address, 0, len(items))
	for _, item := range items {
		result = append(result, shopAddress(item))
	}
	return result, nil
}

func (l *Logic) SaveAddress(ctx context.Context, input appmodel.SaveAddress) (appmodel.Address, error) {
	if l.customer == nil {
		return appmodel.Address{}, customermodel.ErrUnavailable
	}
	item, _, err := l.customer.SaveAddress(ctx, customermodel.SaveAddressCommand{
		Tenant: customerTenant(ctx), Subject: authctx.Caller(ctx).Subject, CommandKey: input.CommandKey,
		ExpectedVersion: input.ExpectedVersion,
		Address: customermodel.Address{
			ID: input.ID, Recipient: input.Recipient, Phone: input.Phone, Country: input.Country,
			Province: input.Province, City: input.City, District: input.District, Detail: input.Detail,
			PostalCode: input.PostalCode, IsDefault: input.IsDefault,
		},
	})
	if err != nil {
		return appmodel.Address{}, err
	}
	return shopAddress(item), nil
}

func (l *Logic) DeleteAddress(ctx context.Context, input appmodel.DeleteAddress) error {
	if l.customer == nil {
		return customermodel.ErrUnavailable
	}
	_, err := l.customer.DeleteAddress(ctx, customermodel.DeleteAddressCommand{
		Tenant: customerTenant(ctx), Subject: authctx.Caller(ctx).Subject, AddressID: input.ID,
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
	})
	return err
}

func (l *Logic) ReplaceDefaultAddress(ctx context.Context, input appmodel.ReplaceDefault) (appmodel.Address, error) {
	if l.customer == nil {
		return appmodel.Address{}, customermodel.ErrUnavailable
	}
	item, _, err := l.customer.ReplaceDefault(ctx, customermodel.ReplaceDefaultCommand{
		Tenant: customerTenant(ctx), Subject: authctx.Caller(ctx).Subject, AddressID: input.ID,
		CommandKey: input.CommandKey, ExpectedVersion: input.ExpectedVersion,
	})
	if err != nil {
		return appmodel.Address{}, err
	}
	return shopAddress(item), nil
}

func (l *Logic) Wishlist(ctx context.Context, cursor int64, limit int) ([]appmodel.WishlistItem, error) {
	if l.customer == nil {
		return nil, customermodel.ErrUnavailable
	}
	items, err := l.customer.Wishlist(ctx, customerTenant(ctx), authctx.Caller(ctx).Subject, cursor, limit)
	if err != nil {
		return nil, err
	}
	result := make([]appmodel.WishlistItem, 0, len(items))
	for _, item := range items {
		result = append(result, appmodel.WishlistItem{ProductID: item.ProductID, CreatedAt: item.CreatedAt})
	}
	return result, nil
}

func (l *Logic) AddWishlist(ctx context.Context, input appmodel.AddWishlist) (appmodel.WishlistItem, error) {
	if l.customer == nil {
		return appmodel.WishlistItem{}, customermodel.ErrUnavailable
	}
	item, _, err := l.customer.AddWishlist(ctx, customermodel.AddWishlistCommand{
		Tenant: customerTenant(ctx), Subject: authctx.Caller(ctx).Subject,
		ProductID: input.ProductID, CommandKey: input.CommandKey,
	})
	if err != nil {
		return appmodel.WishlistItem{}, err
	}
	return appmodel.WishlistItem{ProductID: item.ProductID, CreatedAt: item.CreatedAt}, nil
}

func (l *Logic) RemoveWishlist(ctx context.Context, productID int64) error {
	if l.customer == nil {
		return customermodel.ErrUnavailable
	}
	return l.customer.RemoveWishlist(ctx, customermodel.RemoveWishlistCommand{
		Tenant: customerTenant(ctx), Subject: authctx.Caller(ctx).Subject, ProductID: productID,
	})
}

func (l *Logic) LoginSMSRegions(ctx context.Context, shopCode string) (appmodel.SMSRegions, error) {
	if l.shops == nil {
		return appmodel.SMSRegions{}, shopmodel.ErrNotFound
	}
	value, err := l.shops.GetBySlug(ctx, shopCode)
	if err != nil {
		return appmodel.SMSRegions{}, err
	}
	if value.Status != shopmodel.StatusActive {
		return appmodel.SMSRegions{}, shopmodel.ErrNotFound
	}
	empty := appmodel.SMSRegions{Items: []appmodel.SMSRegion{}}
	if l.grants == nil {
		return empty, nil
	}
	granted, err := l.grants.SMSRegions(ctx, value.MerchantID, value.ID)
	if err != nil {
		return empty, nil
	}
	out := appmodel.SMSRegions{Items: make([]appmodel.SMSRegion, 0, len(granted.Regions)), Unrestricted: granted.Unrestricted}
	for _, item := range granted.Regions {
		if granted.Unrestricted || item.Enabled {
			out.Items = append(out.Items, appmodel.SMSRegion{DialCode: item.DialCode, Name: item.Name, ISO2: item.ISO2, Emoji: item.Emoji})
		}
	}
	if len(out.Items) == 0 {
		for _, code := range granted.DialCodes {
			out.Items = append(out.Items, appmodel.SMSRegion{DialCode: code})
		}
	}
	return out, nil
}

func (l *Logic) Profile(ctx context.Context) (appmodel.Profile, error) {
	claims := authctx.Caller(ctx)
	signedIn := claims.PrincipalType == principal.TypeCustomer
	displayName := "游客"
	if signedIn {
		displayName = "已登录"
	}
	return appmodel.Profile{
		Subject: claims.Subject, PrincipalType: claims.PrincipalType.String(), SignedIn: signedIn, DisplayName: displayName,
	}, nil
}

func (l *Logic) Aftersales(ctx context.Context, query appmodel.AftersaleQuery) (appmodel.AftersalePage, error) {
	if l.aftersales == nil {
		return appmodel.AftersalePage{}, fulfillmentmodel.ErrAftersaleUnavailable
	}
	claims := authctx.Caller(ctx)
	page, err := l.aftersales.List(ctx, fulfillmentmodel.AftersaleQuery{
		MerchantID: claims.MerchantID, ShopID: claims.ShopID, CustomerSubject: claims.Subject,
		Status: fulfillmentmodel.AftersaleStatus(query.Status), Type: fulfillmentmodel.AftersaleType(query.Type),
		Page: query.Page, PageSize: query.PageSize,
	})
	if err != nil {
		return appmodel.AftersalePage{}, err
	}
	out := appmodel.AftersalePage{Items: make([]appmodel.Aftersale, 0, len(page.Items)), Page: page.Page, PageSize: page.PageSize, Total: page.Total}
	for _, item := range page.Items {
		out.Items = append(out.Items, shopAftersale(item))
	}
	return out, nil
}

func (l *Logic) Aftersale(ctx context.Context, aftersaleID int64) (appmodel.Aftersale, error) {
	if l.aftersales == nil {
		return appmodel.Aftersale{}, fulfillmentmodel.ErrAftersaleUnavailable
	}
	claims := authctx.Caller(ctx)
	value, err := l.aftersales.Get(ctx, claims.MerchantID, claims.ShopID, aftersaleID)
	if err != nil {
		return appmodel.Aftersale{}, err
	}
	if value.CustomerSubject != claims.Subject {
		return appmodel.Aftersale{}, fulfillmentmodel.ErrAftersaleNotFound
	}
	return shopAftersale(value), nil
}

func customerTenant(ctx context.Context) customermodel.Tenant {
	claims := authctx.Caller(ctx)
	return customermodel.Tenant{MerchantID: claims.MerchantID, ShopID: claims.ShopID}
}

func shopAddress(item customermodel.Address) appmodel.Address {
	return appmodel.Address{
		ID: item.ID, Recipient: item.Recipient, Phone: item.Phone, Country: item.Country,
		Province: item.Province, City: item.City, District: item.District, Detail: item.Detail,
		PostalCode: item.PostalCode, IsDefault: item.IsDefault, Version: item.Version,
	}
}

func shopAftersale(item fulfillmentmodel.Aftersale) appmodel.Aftersale {
	out := appmodel.Aftersale{
		ID: item.ID, OrderID: item.OrderID, PaymentNo: item.PaymentNo, Type: string(item.Type),
		RequestedAmount: item.RequestedAmount, Amount: item.Amount, Reason: item.Reason,
		Status: string(item.Status), ReturnStatus: string(item.ReturnStatus), HandleNote: item.HandleNote,
		Items: make([]appmodel.AftersaleItem, 0, len(item.Items)), Version: item.Version,
		CreatedAt: item.CreatedAt.UTC().Format(time.RFC3339), UpdatedAt: item.UpdatedAt.UTC().Format(time.RFC3339),
	}
	for _, line := range item.Items {
		out.Items = append(out.Items, appmodel.AftersaleItem{
			ID: line.ID, SKUID: line.SKUID, Title: line.Title, Quantity: line.Quantity,
			RefundAmount: line.RefundAmount, ReceivedQuantity: line.ReceivedQuantity,
		})
	}
	if item.ReviewedAt != nil && !item.ReviewedAt.IsZero() {
		formatted := item.ReviewedAt.UTC().Format(time.RFC3339)
		out.ReviewedAt = &formatted
	}
	if item.ReceivedAt != nil && !item.ReceivedAt.IsZero() {
		formatted := item.ReceivedAt.UTC().Format(time.RFC3339)
		out.ReceivedAt = &formatted
	}
	return out
}
