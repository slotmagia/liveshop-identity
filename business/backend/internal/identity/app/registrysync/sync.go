package registrysync

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	platformv1 "github.com/liveshop-platform/contracts/gen/go/platform/v1"
	"github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/config"
	mysqlrepo "github.com/lvtuopen-ai/liveshop-identity/business/backend/internal/identity/data/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"os"
	"time"
)

type Syncer struct {
	client   platformv1.PlatformRegistryServiceClient
	repo     *mysqlrepo.AuthorizationRepository
	interval time.Duration
	conn     *grpc.ClientConn
}

func New(c config.PlatformRegistry, repo *mysqlrepo.AuthorizationRepository) (*Syncer, error) {
	cert, e := tls.LoadX509KeyPair(c.TLS.CertificateFile, c.TLS.PrivateKeyFile)
	if e != nil {
		return nil, e
	}
	ca, e := os.ReadFile(c.TLS.ClientCAFile)
	if e != nil {
		return nil, e
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(ca) {
		return nil, fmt.Errorf("identity: invalid platform registry CA")
	}
	conn, e := grpc.NewClient(c.Endpoint, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, RootCAs: pool, ServerName: c.ServerName})))
	if e != nil {
		return nil, e
	}
	interval, e := c.Interval()
	if e != nil {
		conn.Close()
		return nil, e
	}
	return &Syncer{client: platformv1.NewPlatformRegistryServiceClient(conn), repo: repo, interval: interval, conn: conn}, nil
}
func (s *Syncer) Once(ctx context.Context) error {
	call, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	snapshot, e := s.client.GetActiveCapabilitySnapshot(call, &platformv1.GetActiveCapabilitySnapshotRequest{})
	if e != nil {
		return e
	}
	return s.repo.ReplaceRegistrySnapshot(ctx, snapshot)
}
func (s *Syncer) Run(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_ = s.Once(ctx)
		}
	}
}
func (s *Syncer) Close() error { return s.conn.Close() }
