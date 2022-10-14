package servers

import (
	"net"
	"net/http"

	"github.com/couchbase/stellar-nebula/genproto/query_v1"
	"github.com/gorilla/mux"
	"go.uber.org/zap"
)

type QueryServerOptions struct {
	Logger *zap.Logger

	QueryServer query_v1.QueryServer
}

type QueryServer struct {
	logger      *zap.Logger
	queryServer query_v1.QueryServer

	httpServer *http.Server
}

func NewQueryServer(opts *QueryServerOptions) (*QueryServer, error) {
	s := &QueryServer{
		logger:      opts.Logger,
		queryServer: opts.QueryServer,
	}

	router := mux.NewRouter()

	router.HandleFunc("/", s.handleRoot)

	s.httpServer = &http.Server{
		Handler: router,
	}

	return s, nil
}

func (s *QueryServer) handleRoot(w http.ResponseWriter, r *http.Request) {
	// TODO(brett19): Handle errors and stuff here...
	w.WriteHeader(200)
	w.Write([]byte("mgmt service"))
}

func (s *QueryServer) Serve(l net.Listener) error {
	return s.httpServer.Serve(l)
}
