package master

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"reflect"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/snowflake"
	"github.com/golang/protobuf/ptypes/empty"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/registry"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.etcd.io/etcd/client/v3/concurrency"
	"go.uber.org/zap"
	"golang.org/x/net/context"

	proto "github.com/wenzapen/crawler/proto/crawler"
)

const (
	RESOURCEPATH = "/resources"

	ADDRESOURCE = iota
	DELETERESOURCE

	ServiceName = "go.micro.server.worker"
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

func (m *Master) SetForwardClient(cli proto.CrawlerMasterService) {
	m.forwardCli = cli
}

func (m *Master) DeleteResource(ctx context.Context, spec *proto.ResourceSpec, empty *empty.Empty) error {
	if !m.IsLeader() && m.leaderID != "" && m.leaderID != m.ID {
		addr := getLeaderAddress(m.leaderID)
		_, err := m.forwardCli.DeleteResource(ctx, spec, client.WithAddress(addr))
		return err
	}

	m.rlock.Lock()
	defer m.rlock.Unlock()

	r, ok := m.resources[spec.Name]
	if !ok {
		return fmt.Errorf("resource %s not found", spec.Name)
	}

	if _, err := m.etcdCli.Delete(context.Background(), getResourcePath(spec.Name)); err != nil {
		return err
	}

	delete(m.resources, spec.Name)

	if r.AssignedNode != "" {
		nodeID, err := getNodeID(r.AssignedNode)
		if err != nil {
			return err
		}
		if ns, ok := m.workNodes[nodeID]; ok {
			ns.Workload--
		}
	}

	return nil
}

func (m *Master) AddResource(ctx context.Context, spec *proto.ResourceSpec, resp *proto.NodeSpec) error {
	if !m.IsLeader() && m.leaderID != "" && m.leaderID != m.ID {
		addr := getLeaderAddress(m.leaderID)
		nodeSpec, err := m.forwardCli.AddResource(ctx, spec, client.WithAddress(addr))
		resp.Id = nodeSpec.Id
		resp.Address = nodeSpec.Address
		return err
	}
	m.rlock.Lock()
	defer m.rlock.Unlock()
	NodeSpec, err := m.addResource(&ResourceSpec{Name: spec.Name})
	if NodeSpec != nil {
		resp.Id = NodeSpec.Node.Id
		resp.Address = NodeSpec.Node.Address
	}
	return err

	//Output:

}
func getLeaderAddress(address string) string {
	s := strings.Split(address, "-")
	if len(s) < 2 {
		return ""
	}
	return s[1]
}

func New(id string, opts ...Option) (*Master, error) {
	options := DefaultOptions
	for _, o := range opts {
		o(&options)
	}

	m := &Master{}
	m.options = options
	m.resources = make(map[string]*ResourceSpec)

	node, err := snowflake.NewNode(1)
	if err != nil {
		return nil, err
	}
	m.IDGen = node
	ipv4, err := getLocalIP()
	if err != nil {
		return nil, err
	}
	m.ID = genMasterID(id, ipv4, m.GRPCAddress)
	m.logger.Sugar().Debugln("master id: ", m.ID)

	endpoints := []string{m.registryURL}
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		return nil, err
	}

	m.etcdCli = cli

	m.updateWorkNodes()
	m.AddSeed()
	go m.Campaign()
	return m, nil

}

func genMasterID(id string, ipv4 string, GPRCAddress string) string {
	return "master" + id + "-" + ipv4 + GPRCAddress
}

func (m *Master) IsLeader() bool {
	return atomic.LoadInt32(&m.ready) != 0
}

func (m *Master) Campaign() {
	s, err := concurrency.NewSession(m.etcdCli, concurrency.WithTTL(5))
	if err != nil {
		fmt.Println("new session failed", err)
	}
	defer s.Close()

	e := concurrency.NewElection(s, "/crawler/election")
	leaderCh := make(chan error)
	go m.elect(e, leaderCh)
	leaderChange := e.Observe(context.Background())

	resp := <-leaderChange
	m.logger.Info("watch leader change", zap.String("leader", string(resp.Kvs[0].Value)))
	m.leaderID = string(resp.Kvs[0].Value)

	workerNodeChange := m.WatchWorker()

	for {
		select {
		case err := <-leaderCh:
			if err != nil {
				m.logger.Error("leader elect failed", zap.Error(err))
				go m.elect(e, leaderCh)
			} else {
				m.logger.Info("start to become leader")
				m.leaderID = m.ID
				if !m.IsLeader() {
					if err := m.BecomeLeader(); err != nil {
						m.logger.Error("become leader failed", zap.Error(err))
					}
				}
			}
		case res := <-leaderChange:
			if len(res.Kvs) > 0 {
				m.logger.Info("leader change", zap.String("leader", string(res.Kvs[0].Value)))
				m.leaderID = string(res.Kvs[0].Value)
				if m.ID != m.leaderID {
					atomic.StoreInt32(&m.ready, 0)
				}
			}

		case res := <-workerNodeChange:
			m.logger.Info("worker node change", zap.Any("res", res))
			m.updateWorkNodes()
			if err := m.loadResource(); err != nil {
				m.logger.Error("load resource failed", zap.Error(err))
			}
			m.reAssign()

		case <-time.After(20 * time.Second):
			resp, err := e.Leader(context.Background())
			if err != nil {
				m.logger.Error("get leader failed", zap.Error(err))
				if errors.Is(err, concurrency.ErrElectionNoLeader) {
					go m.elect(e, leaderCh)

				}
			}
			if resp != nil && len(resp.Kvs) > 0 {
				m.logger.Debug("get leader", zap.String("value", string(resp.Kvs[0].Value)))
				m.leaderID = string(resp.Kvs[0].Value)
				if m.leaderID != m.ID {
					atomic.StoreInt32(&m.ready, 0)
				}
			}
		}

	}

}

func (m *Master) elect(e *concurrency.Election, ch chan error) {
	err := e.Campaign(context.Background(), m.ID)
	ch <- err
}

func (m *Master) WatchWorker() chan *registry.Result {
	watcher, err := m.regisry.Watch(registry.WatchService(ServiceName))
	if err != nil {
		panic(err)
	}
	ch := make(chan *registry.Result)
	go func() {
		for {
			res, err := watcher.Next()
			if err != nil {
				m.logger.Error("watcher next failed", zap.Error(err))
				continue
			}
			ch <- res
		}
	}()
	return ch
}

func (m *Master) BecomeLeader() error {
	m.updateWorkNodes()
	if err := m.loadResource(); err != nil {
		return fmt.Errorf("load resource failed: %w", err)
	}
	m.reAssign()
	atomic.StoreInt32(&m.ready, 1)
	return nil
}

func (m *Master) updateWorkNodes() {
	service, err := m.regisry.GetService(ServiceName)
	if err != nil {
		m.logger.Error("get worker service failed", zap.Error(err))

	}

	m.rlock.Lock()
	defer m.rlock.Unlock()
	nodes := make(map[string]*NodeSpec)
	if len(service) > 0 {
		for _, spec := range service[0].Nodes {
			nodes[spec.Id] = &NodeSpec{
				Node: spec,
			}

		}
	}
	added, deleted, changed := workloadDiff(m.workNodes, nodes)
	m.logger.Sugar().Info("work joined: ", added, "work leaved: ", deleted, "work changed: ", changed)
	m.workNodes = nodes
}

type Command int

const (
	MSGADD Command = iota
	MSGDELETE
)

type Message struct {
	Cmd   Command
	Specs []*ResourceSpec
}

type NodeSpec struct {
	Node     *registry.Node
	Workload int
}

type ResourceSpec struct {
	ID           string
	Name         string
	AssignedNode string
	CreationTime int64
}

func getResourcePath(name string) string {
	return RESOURCEPATH + "/" + name
}

func encode(s *ResourceSpec) string {
	ds, _ := json.Marshal(s)
	return string(ds)
}

func Decode(ds []byte) (*ResourceSpec, error) {
	var s *ResourceSpec
	err := json.Unmarshal(ds, &s)
	return s, err
}

func (m *Master) addResource(r *ResourceSpec) (*NodeSpec, error) {
	r.ID = m.IDGen.Generate().String()
	nd, err := m.Assign(r)
	if err != nil {
		m.logger.Error("assign resource failed", zap.Error(err))
		return nil, err
	}
	if nd.Node == nil {
		m.logger.Error("no node to assign", zap.Error(err))
		return nil, errors.New("no node to assign")
	}
	r.AssignedNode = nd.Node.Id + "|" + nd.Node.Address
	r.CreationTime = time.Now().UnixNano()
	m.logger.Debug("add resource", zap.Any("specs", r))
	_, err = m.etcdCli.Put(context.Background(), getResourcePath(r.Name), encode(r))
	if err != nil {
		m.logger.Error("put etcd failed", zap.Error(err))
		return nil, err
	}
	m.resources[r.Name] = r
	nd.Workload++
	return nd, nil
}

func (m *Master) Assign(r *ResourceSpec) (*NodeSpec, error) {
	candidates := make([]*NodeSpec, 0, len(m.workNodes))
	for _, node := range m.workNodes {
		candidates = append(candidates, node)
	}
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].Workload < candidates[j].Workload
	})
	if len(candidates) > 0 {
		return candidates[0], nil
	}
	return nil, errors.New("no worker available")
}

func (m *Master) AddSeed() {
	rs := make([]*ResourceSpec, 0, len(m.Seeds))

	for _, seed := range m.Seeds {
		resp, err := m.etcdCli.Get(context.Background(), getResourcePath(seed.Name), clientv3.WithPrefix(), clientv3.WithSerializable())
		if err != nil {
			m.logger.Error("get resource failed", zap.Error(err))
			continue
		}
		if len(resp.Kvs) == 0 {
			r := &ResourceSpec{
				Name: seed.Name,
			}
			rs = append(rs, r)
		}

	}
	for _, r := range rs {
		m.addResource(r)
	}
}

func (m *Master) loadResource() error {
	resp, err := m.etcdCli.Get(context.Background(), RESOURCEPATH, clientv3.WithPrefix(), clientv3.WithSerializable())
	if err != nil {
		return fmt.Errorf("etcd get resource failed: %v", err)
	}

	resources := make(map[string]*ResourceSpec)
	for _, kv := range resp.Kvs {
		s, err := Decode(kv.Value)
		if err == nil && s != nil {
			resources[s.Name] = s
		}

	}
	m.logger.Info("leader init load resources", zap.Int("length:", len(resources)))
	m.rlock.Lock()
	defer m.rlock.Unlock()
	m.resources = resources

	for _, r := range m.resources {
		if r.AssignedNode != "" {
			id, err := getNodeID(r.AssignedNode)
			if err != nil {
				m.logger.Error("getNodeID failed", zap.Error(err))
			}
			node, ok := m.workNodes[id]
			if ok {
				node.Workload++
			}
		}
	}
	return nil
}

func (m *Master) reAssign() {
	rs := make([]*ResourceSpec, 0, len(m.resources))
	m.rlock.Lock()
	defer m.rlock.Unlock()

	for _, r := range m.resources {
		if r.AssignedNode == "" {
			rs = append(rs, r)
			continue
		}
		id, err := getNodeID(r.AssignedNode)
		if err != nil {
			m.logger.Error("get nodeid failed", zap.Error(err))
		}
		if _, ok := m.workNodes[id]; !ok {
			rs = append(rs, r)
		}
	}
	for _, r := range rs {
		m.addResource(r)
	}
}

func getNodeID(assigned string) (string, error) {
	node := strings.Split(assigned, "|")
	if len(node) < 2 {
		return "", errors.New("invalid assigned node")
	}
	id := node[0]
	return id, nil
}

func workloadDiff(old map[string]*NodeSpec, new map[string]*NodeSpec) ([]string, []string, []string) {
	added := make([]string, 0)
	deleted := make([]string, 0)
	changed := make([]string, 0)

	for k, v := range new {
		if ov, ok := old[k]; ok {
			if !reflect.DeepEqual(ov, v) {
				changed = append(changed, k)
			}
		} else {
			added = append(added, k)
		}
	}
	for k, _ := range old {
		if _, ok := new[k]; !ok {
			deleted = append(deleted, k)
		}
	}
	return added, deleted, changed
}

func getLocalIP() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
			return ipNet.IP.String(), nil
		}
	}

	return "", nil
}
