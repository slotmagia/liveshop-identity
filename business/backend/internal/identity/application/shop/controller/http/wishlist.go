package http

import (
	"context"

	wishlistapi "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/api/http/v1/wishlist"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/appmodel"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/application/shop/service"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/common/web"
)

type WishlistReader struct{ service service.Shop }
type WishlistWriter struct{ service service.Shop }

func NewWishlistReader(application service.Shop) *WishlistReader { return &WishlistReader{application} }
func NewWishlistWriter(application service.Shop) *WishlistWriter { return &WishlistWriter{application} }

func (c *WishlistReader) List(ctx context.Context, request *wishlistapi.ListReq) (*wishlistapi.ListRes, error) {
	items, err := c.service.Wishlist(ctx, request.Cursor, request.Limit)
	if err != nil {
		return nil, web.Failure(err)
	}
	result := &wishlistapi.ListRes{Items: make([]wishlistapi.Item, 0, len(items))}
	for _, item := range items {
		result.Items = append(result.Items, wishlistapi.Item{ProductID: item.ProductID, CreatedAt: item.CreatedAt})
	}
	return result, nil
}

func (c *WishlistWriter) Create(ctx context.Context, request *wishlistapi.CreateReq) (*wishlistapi.CreateRes, error) {
	item, err := c.service.AddWishlist(ctx, appmodel.AddWishlist{ProductID: request.ProductID, CommandKey: request.CommandKey})
	if err != nil {
		return nil, web.Failure(err)
	}
	return &wishlistapi.CreateRes{ProductID: item.ProductID, CreatedAt: item.CreatedAt}, nil
}

func (c *WishlistWriter) Delete(ctx context.Context, request *wishlistapi.DeleteReq) (*wishlistapi.DeleteRes, error) {
	if err := c.service.RemoveWishlist(ctx, request.ProductID); err != nil {
		return nil, web.Failure(err)
	}
	return &wishlistapi.DeleteRes{OK: true}, nil
}
