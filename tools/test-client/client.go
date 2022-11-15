package gocbps

import (
	"crypto/x509"
	"strings"

	"github.com/couchbase/stellar-nebula/genproto/view_v1"

	"github.com/couchbase/stellar-nebula/genproto/analytics_v1"
	"github.com/couchbase/stellar-nebula/genproto/kv_v1"
	"github.com/couchbase/stellar-nebula/genproto/query_v1"
	"github.com/couchbase/stellar-nebula/genproto/routing_v1"
	"github.com/couchbase/stellar-nebula/genproto/search_v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn            *grpc.ClientConn
	routingClient   routing_v1.RoutingClient
	kvClient        kv_v1.KvClient
	queryClient     query_v1.QueryClient
	searchClient    search_v1.SearchClient
	analyticsClient analytics_v1.AnalyticsClient
	viewClient      view_v1.ViewClient
}

type ConnectOptions struct {
	Username          string
	Password          string
	ClientCertificate *x509.CertPool
}

func Connect(connStr string, opts *ConnectOptions) (*Client, error) {
	var transportDialOpt grpc.DialOption
	var perRpcDialOpt grpc.DialOption

	if opts.ClientCertificate != nil {
		transportDialOpt = grpc.WithTransportCredentials(credentials.NewClientTLSFromCert(opts.ClientCertificate, ""))
		perRpcDialOpt = nil
	} else if opts.Username != "" && opts.Password != "" {
		basicAuthCreds, err := newGrpcBasicAuth(opts.Username, opts.Password)
		if err != nil {
			return nil, err
		}

		transportDialOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
		perRpcDialOpt = grpc.WithPerRPCCredentials(basicAuthCreds)
	} else {
		transportDialOpt = grpc.WithTransportCredentials(insecure.NewCredentials())
		perRpcDialOpt = nil
	}

	// use port 18091 by default
	{
		connStrPieces := strings.Split(connStr, ":")
		if len(connStrPieces) == 1 {
			connStrPieces = append(connStrPieces, "18098")
		}
		connStr = strings.Join(connStrPieces, ":")
	}

	dialOpts := []grpc.DialOption{transportDialOpt}
	if perRpcDialOpt != nil {
		dialOpts = append(dialOpts, perRpcDialOpt)
	}

	conn, err := grpc.Dial(connStr, dialOpts...)
	if err != nil {
		return nil, err
	}

	routingClient := routing_v1.NewRoutingClient(conn)
	kvClient := kv_v1.NewKvClient(conn)
	queryClient := query_v1.NewQueryClient(conn)
	searchClient := search_v1.NewSearchClient(conn)
	analyticsClient := analytics_v1.NewAnalyticsClient(conn)
	viewClient := view_v1.NewViewClient(conn)

	return &Client{
		conn:            conn,
		routingClient:   routingClient,
		kvClient:        kvClient,
		queryClient:     queryClient,
		searchClient:    searchClient,
		analyticsClient: analyticsClient,
		viewClient:      viewClient,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Bucket(bucketName string) *Bucket {
	return &Bucket{
		client:     c,
		bucketName: bucketName,
	}
}

// INTERNAL: Used for testing
func (c *Client) GetConn() *grpc.ClientConn {
	return c.conn
}
