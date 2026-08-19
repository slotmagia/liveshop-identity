package notification

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	platformv1 "github.com/liveshop-platform/contracts/gen/go/platform/v1"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/capability/auth/model"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type Client struct {
	rpc  platformv1.PlatformNotificationServiceClient
	conn *grpc.ClientConn
}

func New(c config.PlatformRegistry) (*Client, error) {
	cert, err := tls.LoadX509KeyPair(c.TLS.CertificateFile, c.TLS.PrivateKeyFile)
	if err != nil {
		return nil, err
	}
	ca, err := os.ReadFile(c.TLS.ClientCAFile)
	if err != nil {
		return nil, err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("identity: invalid platform notification CA")
	}
	conn, err := grpc.NewClient(c.Endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
		MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, RootCAs: pool, ServerName: c.ServerName,
	})))
	if err != nil {
		return nil, err
	}
	return &Client{rpc: platformv1.NewPlatformNotificationServiceClient(conn), conn: conn}, nil
}

func (c *Client) Dispatch(ctx context.Context, message auth.Dispatch) ([]model.Delivery, error) {
	if c == nil || c.rpc == nil {
		return nil, model.ErrUnavailable
	}
	call, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	response, err := c.rpc.Dispatch(call, &platformv1.DispatchRequest{
		EventKey:    message.EventKey,
		DeliveryKey: message.DeliveryKey,
		MerchantId:  message.MerchantID,
		ShopId:      message.ShopID,
		Recipients:  &platformv1.NotificationRecipients{Phone: message.Phone, Email: message.Email},
		Variables:   message.Variables,
	})
	if err != nil {
		return nil, model.ErrDeliveryFailed
	}
	deliveries := make([]model.Delivery, 0, len(response.GetDeliveries()))
	for _, item := range response.GetDeliveries() {
		deliveries = append(deliveries, model.Delivery{Channel: item.GetChannel(), Status: item.GetStatus()})
	}
	return deliveries, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}
