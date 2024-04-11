package master

import (
	"sync"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type Master struct {
	ID         string
	ready      int32
	leaderID   string
	workNodes  map[string]*NodeSpec
	resources  map[string]*ResourceSpec
	IDGen      *snowflake.Node
	etcdCli    *clientv3.Client
	forwardCli proto.CrawlerMasterService
	rlock      sync.Mutex
	options
}
