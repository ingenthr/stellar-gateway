package system

import (
	"context"
	"crypto/tls"
	"sync"

	"go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/status"

	"github.com/couchbase/goprotostellar/genproto/admin_bucket_v1"
	"github.com/couchbase/goprotostellar/genproto/admin_collection_v1"
	"github.com/couchbase/goprotostellar/genproto/admin_query_v1"
	"github.com/couchbase/goprotostellar/genproto/admin_search_v1"
	"github.com/couchbase/goprotostellar/genproto/analytics_v1"
	"github.com/couchbase/goprotostellar/genproto/internal_hooks_v1"
	"github.com/couchbase/goprotostellar/genproto/kv_v1"
	"github.com/couchbase/goprotostellar/genproto/query_v1"
	"github.com/couchbase/goprotostellar/genproto/routing_v1"
	"github.com/couchbase/goprotostellar/genproto/search_v1"
	"github.com/couchbase/goprotostellar/genproto/transactions_v1"
	"github.com/couchbase/stellar-gateway/gateway/dataimpl"
	"github.com/couchbase/stellar-gateway/gateway/hooks"
	"github.com/couchbase/stellar-gateway/gateway/sdimpl"
	"github.com/couchbase/stellar-gateway/pkg/interceptors"
	"github.com/couchbase/stellar-gateway/pkg/metrics"
	"github.com/grpc-ecosystem/go-grpc-middleware/v2/interceptors/recovery"
)

// TODO(brett19): Implement the gateway system as its own component

const maxMsgSize = 25000000 // 25MB

type SystemOptions struct {
	Logger *zap.Logger

	DataImpl *dataimpl.Servers
	SdImpl   *sdimpl.Servers
	Metrics  *metrics.SnMetrics

	TlsConfig *tls.Config
	Debug     bool
}

type System struct {
	logger *zap.Logger

	dataServer *grpc.Server
	sdServer   *grpc.Server
}

func NewSystem(opts *SystemOptions) (*System, error) {
	dataImpl := opts.DataImpl
	sdImpl := opts.SdImpl

	hooksManager := hooks.NewHooksManager(opts.Logger.Named("hooks-manager"))
	debugInterceptor := interceptors.NewDebugInterceptor(opts.Logger.Named("grpc-debug"))
	metricsInterceptor := interceptors.NewMetricsInterceptor(opts.Metrics)

	recoveryHandler := func(p any) (err error) {
		opts.Logger.Error("a panic has been triggered", zap.Any("error: ", p))
		return status.Errorf(codes.Internal, "An internal error occurred.")
	}

	var unaryInterceptors []grpc.UnaryServerInterceptor
	unaryInterceptors = append(unaryInterceptors, otelgrpc.UnaryServerInterceptor())
	unaryInterceptors = append(unaryInterceptors, metricsInterceptor.UnaryInterceptor())
	if opts.Debug {
		unaryInterceptors = append(unaryInterceptors, debugInterceptor.UnaryInterceptor())
	}
	unaryInterceptors = append(unaryInterceptors, hooksManager.UnaryInterceptor())
	unaryInterceptors = append(unaryInterceptors, recovery.UnaryServerInterceptor(
		recovery.WithRecoveryHandler(recoveryHandler),
	))

	var streamInterceptors []grpc.StreamServerInterceptor
	streamInterceptors = append(streamInterceptors, otelgrpc.StreamServerInterceptor())
	streamInterceptors = append(streamInterceptors, metricsInterceptor.StreamInterceptor())
	if opts.Debug {
		streamInterceptors = append(streamInterceptors, debugInterceptor.StreamInterceptor())
	}
	streamInterceptors = append(streamInterceptors, recovery.StreamServerInterceptor(
		recovery.WithRecoveryHandler(recoveryHandler),
	))

	// TODO(abose): Same serverOpts passed; need to break into two, if needed.
	serverOpts := []grpc.ServerOption{
		grpc.ChainUnaryInterceptor(unaryInterceptors...),
		grpc.ChainStreamInterceptor(streamInterceptors...),
		grpc.Creds(credentials.NewTLS(opts.TlsConfig)),
		grpc.MaxRecvMsgSize(maxMsgSize),
	}

	dataSrv := grpc.NewServer(serverOpts...)

	internal_hooks_v1.RegisterHooksServiceServer(dataSrv, hooksManager.Server())
	kv_v1.RegisterKvServiceServer(dataSrv, dataImpl.KvV1Server)
	query_v1.RegisterQueryServiceServer(dataSrv, dataImpl.QueryV1Server)
	search_v1.RegisterSearchServiceServer(dataSrv, dataImpl.SearchV1Server)
	analytics_v1.RegisterAnalyticsServiceServer(dataSrv, dataImpl.AnalyticsV1Server)
	admin_bucket_v1.RegisterBucketAdminServiceServer(dataSrv, dataImpl.AdminBucketV1Server)
	admin_collection_v1.RegisterCollectionAdminServiceServer(dataSrv, dataImpl.AdminCollectionV1Server)
	admin_search_v1.RegisterSearchAdminServiceServer(dataSrv, dataImpl.AdminSearchIndexV1Server)
	admin_query_v1.RegisterQueryAdminServiceServer(dataSrv, dataImpl.AdminQueryIndexV1Server)
	transactions_v1.RegisterTransactionsServiceServer(dataSrv, dataImpl.TransactionsV1Server)

	// health check
	grpc_health_v1.RegisterHealthServer(dataSrv, HealthV1Server{})

	sdSrv := grpc.NewServer(serverOpts...)

	internal_hooks_v1.RegisterHooksServiceServer(sdSrv, hooksManager.Server())
	routing_v1.RegisterRoutingServiceServer(sdSrv, sdImpl.RoutingV1Server)

	s := &System{
		logger:     opts.Logger,
		dataServer: dataSrv,
		sdServer:   sdSrv,
	}

	return s, nil
}

func (s *System) Serve(ctx context.Context, l *Listeners) error {
	var wg sync.WaitGroup

	go func() {
		<-ctx.Done()
		s.dataServer.Stop()
		s.sdServer.Stop()
	}()

	if l.dataListener != nil {
		wg.Add(1)
		go func() {
			err := s.dataServer.Serve(l.dataListener)
			if err != nil {
				s.logger.Warn("data server serve failed", zap.Error(err))
			}
			wg.Done()
		}()
	}

	if l.sdListener != nil {
		wg.Add(1)
		go func() {
			err := s.sdServer.Serve(l.sdListener)
			if err != nil {
				s.logger.Warn("service discovery server serve failed", zap.Error(err))
			}
			wg.Done()
		}()
	}

	wg.Wait()
	return nil
}

func (s *System) Shutdown() {
	if s.dataServer != nil {
		s.dataServer.GracefulStop()
	}

	if s.sdServer != nil {
		s.sdServer.GracefulStop()
	}
}
