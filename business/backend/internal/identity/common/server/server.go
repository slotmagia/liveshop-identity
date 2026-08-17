// Package server is the identity HTTP composition root. It knows how to
// mount a surface and nothing about what any surface does.
package server

import (
	"context"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/gogf/gf/v2/net/ghttp"
)

// serverSequence names each engine uniquely. GoFrame keys servers by name, so a
// fixed name would make two instances in one process share routes and break
// parallel tests.
var serverSequence atomic.Uint64

// Surface is one page-backing surface contributed to the module server. The
// surface owns its own prefix, middleware and bindings.
type Surface struct {
	Name     string
	Register func(root *ghttp.RouterGroup)
}

type Server struct {
	engine *ghttp.Server
}

// New builds an isolated engine per call so parallel tests never share routes.
func New(address string, surfaces []Surface) *Server {
	engine := ghttp.GetServer(fmt.Sprintf("identity-http-%d", serverSequence.Add(1)))
	engine.SetAddr(address)
	engine.SetDumpRouterMap(false)
	engine.SetAccessLogEnabled(false)
	engine.SetReadTimeout(15 * time.Second)
	engine.Group("/", func(root *ghttp.RouterGroup) {
		for _, surface := range surfaces {
			surface.Register(root)
		}
	})
	return &Server{engine: engine}
}

// Run serves until the context is cancelled, then drains in the background.
func (s *Server) Run(ctx context.Context) error {
	if err := s.engine.Start(); err != nil {
		return err
	}
	<-ctx.Done()
	return s.engine.Shutdown()
}

// Port reports the bound port once the server is listening. Tests bind to :0 so
// they can run in parallel, and need this to address the instance.
func (s *Server) Port() int { return s.engine.GetListenedPort() }
