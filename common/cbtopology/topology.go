package cbtopology

type Server struct {
	HostID      string
	NodeID      string
	ServerGroup string

	HasMgmt      bool
	HasKv        bool
	HasViews     bool
	HasQuery     bool
	HasAnalytics bool
	HasSearch    bool
}

type DataServer struct {
	Server *Server

	Vbuckets        []int
	VbucketReplicas []int
}

type Topology struct {
	RevEpoch uint64
	Revision uint64

	Servers []*Server
}

type BucketTopology struct {
	RevEpoch uint64
	Revision uint64

	Servers     []*Server
	DataServers []*DataServer
}
