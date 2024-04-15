package engine

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"github.com/wenzapen/crawler/master"
	"github.com/wenzapen/crawler/parse/doubanbook"
	"github.com/wenzapen/crawler/parse/doubangroup"
	"github.com/wenzapen/crawler/spider"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
)

var Store = &CrawlerStore{
	list: []*spider.Task{},
	Hash: make(map[string]*spider.Task),
}

func init() {
	Store.Add(doubangroup.DoubanGroupTask)
	Store.Add(doubanbook.DoubanBookTask)
}

type CrawlerStore struct {
	list []*spider.Task
	Hash map[string]*spider.Task
}

func (c *CrawlerStore) Add(task *spider.Task) {
	c.Hash[task.Name] = task
	c.list = append(c.list, task)
}

type Crawler struct {
	id          string
	out         chan spider.ParseResult
	Visited     map[string]bool
	VisitedLock sync.Mutex

	failures    map[string]*spider.Request
	failureLock sync.Mutex

	resources map[string]*master.ResourceSpec
	rlock     sync.Mutex

	etcdCli *clientv3.Client
	options
}

func GetFields(taskName string, ruleName string) []string {
	return Store.Hash[taskName].Rule.Trunk[ruleName].ItemFields
}

type Scheduler interface {
	Schedule()
	Push(...*spider.Request)
	Pull() *spider.Request
}

type Schedule struct {
	requestChan chan *spider.Request
	workerChan  chan *spider.Request
	priReqQueue []*spider.Request
	reqQueue    []*spider.Request
	Logger      *zap.Logger
}

func NewEngine(opts ...Option) (*Crawler, error) {
	options := DefaultOptions
	for _, opt := range opts {
		opt(&options)
	}

	e := &Crawler{}
	e.options = options
	e.Visited = make(map[string]bool, 100)
	e.failures = make(map[string]*spider.Request)
	e.out = make(chan spider.ParseResult)

	for _, task := range Store.list {
		task.Fetcher = e.Fetcher
		task.Storage = e.Storage
	}

	endpoints := []string{e.resitryURL}
	cli, err := clientv3.New(clientv3.Config{Endpoints: endpoints})
	if err != nil {
		return nil, err
	}
	e.etcdCli = cli

	return e, nil
}

func NewSchedule() *Schedule {
	s := Schedule{}
	s.requestChan = make(chan *spider.Request)
	s.workerChan = make(chan *spider.Request)
	return &s
}

func (s *Schedule) Push(reqs ...*spider.Request) {
	for _, req := range reqs {
		s.requestChan <- req
	}
}

func (s *Schedule) Pull() *spider.Request {
	r := <-s.workerChan
	return r
}

func (s *Schedule) Output() *spider.Request {
	r := <-s.workerChan
	return r
}

func (s *Schedule) Schedule() {
	var req *spider.Request
	var workerCh chan *spider.Request
	for {

		if req == nil && len(s.priReqQueue) > 0 {
			req = s.priReqQueue[0]
			s.priReqQueue = s.priReqQueue[1:]
			workerCh = s.workerChan
		}
		if req == nil && len(s.reqQueue) > 0 {
			req = s.reqQueue[0]
			s.reqQueue = s.reqQueue[1:]
			workerCh = s.workerChan
		}
		select {
		case workerCh <- req:
			req = nil
			workerCh = nil
		case r := <-s.requestChan:
			if r.Priority > 0 {
				s.priReqQueue = append(s.priReqQueue, r)
			} else {
				s.reqQueue = append(s.reqQueue, r)
			}

		}
	}
}

func (c *Crawler) Run(id string, cluster bool) {
	c.id = id
	if !cluster {
		c.HandleSeeds()
	}
	go c.loadResource()
	go c.watchResources()
	go c.Schedule()
	for i := 0; i < c.WorkCount; i++ {
		go c.CreateWorker()
	}
	c.HandleResult()
}

func (e *Crawler) Schedule() {

	e.scheduler.Schedule()

}

func (e *Crawler) HandleResult() {
	for {
		result := <-e.out
		for _, item := range result.Items {
			switch d := item.(type) {
			case *spider.DataCell:
				if err := d.Task.Storage.Save(d); err != nil {
					e.Logger.Error("")
				}
			}
			e.Logger.Sugar().Info("get result ", item)
		}

	}
}

func (e *Crawler) CreateWorker() {
	for {
		req := e.scheduler.Pull()
		if err := req.Check(); err != nil {
			e.Logger.Error("check failed", zap.Error(err))
			continue
		}
		if e.HasVisited(req) {
			e.Logger.Debug("request has visited", zap.String("url:", req.URL))
			continue
		}
		e.StoreVisited(req)
		body, err := req.Task.Fetcher.Get(req)
		if err != nil {
			e.Logger.Error("can't fetch ", zap.Error(err), zap.String("url:", req.URL))
			e.SetFailure(req)
			continue
		}
		rule := req.Task.Rule.Trunk[req.RuleName]
		ctx := &spider.Context{
			Body: body,
			Req:  req,
		}
		result, err := rule.ParseFunc(ctx)
		if err != nil {
			e.Logger.Error("ParseFunc failed", zap.Error(err), zap.String("url: ", req.URL))
			continue
		}
		if len(result.Requests) > 0 {
			go e.scheduler.Push(result.Requests...)
		}
		e.out <- result
	}
}

func (e *Crawler) HasVisited(r *spider.Request) bool {
	e.VisitedLock.Lock()
	defer e.VisitedLock.Unlock()
	unique := r.Unique()
	return e.Visited[unique]
}

func (e *Crawler) StoreVisited(reqs ...*spider.Request) {
	e.VisitedLock.Lock()
	defer e.VisitedLock.Unlock()
	for _, r := range reqs {
		unique := r.Unique()
		e.Visited[unique] = true
	}
}

func (c *Crawler) SetFailure(req *spider.Request) {
	if !req.Task.Reload {
		c.VisitedLock.Lock()
		defer c.VisitedLock.Unlock()
		unique := req.Unique()
		delete(c.Visited, unique)
	}
	c.failureLock.Lock()
	defer c.failureLock.Unlock()
	if _, ok := c.failures[req.Unique()]; !ok {
		c.failures[req.Unique()] = req
		c.scheduler.Push(req)
	}

}

func (c *Crawler) HandleSeeds() {
	var reqs []*spider.Request
	for _, task := range c.Seeds {
		t, ok := Store.Hash[task.Name]
		if !ok {
			c.Logger.Error("cannot find predefined task", zap.String("task name", task.Name))
			continue
		}
		task.Rule = t.Rule
		rootReqs, err := task.Rule.Root()
		if err != nil {
			c.Logger.Error("get root failed", zap.Error(err))
			continue
		}
		for _, req := range rootReqs {
			req.Task = task
		}
		reqs = append(reqs, rootReqs...)
	}
	go c.scheduler.Push(reqs...)
}

func (c *Crawler) watchResources() {
	watch := c.etcdCli.Watch(context.Background(), master.RESOURCEPATH, clientv3.WithPrefix(), clientv3.WithPrevKV())
	for w := range watch {
		if w.Err() != nil {
			c.Logger.Error("watch resource failed", zap.Error(w.Err()))
			continue
		}
		if w.Canceled {
			c.Logger.Error("watch resource canceled")
			return

		}
		for _, ev := range w.Events {
			switch ev.Type {
			case clientv3.EventTypePut:
				spec, err := master.Decode(ev.Kv.Value)
				if err != nil {
					c.Logger.Error("decode etcd value failed", zap.Error(err))
				}
				if ev.IsCreate() {
					c.Logger.Info("receive create resource", zap.Any("spec", spec))
				} else if ev.IsModify() {
					c.Logger.Info("receive update resource", zap.Any("spec", spec))
				}

				c.rlock.Lock()
				c.runTask(spec.Name)
				c.rlock.Unlock()

			case clientv3.EventTypeDelete:
				spec, err := master.Decode(ev.PrevKv.Value)
				c.Logger.Info("receive delete resouce", zap.Any("spec", spec))
				if err != nil {
					c.Logger.Error("decode etcd value failed", zap.Error(err))
				}
				c.rlock.Lock()

				c.deleteTask(spec.Name)
				c.rlock.Unlock()
			}
		}
	}
}

func getID(assignedNode string) string {
	s := strings.Split(assignedNode, "|")
	if len(s) < 2 {
		return ""

	}
	return s[0]
}

func (c *Crawler) loadResource() error {
	resp, err := c.etcdCli.Get(context.Background(), master.RESOURCEPATH, clientv3.WithPrefix(), clientv3.WithSerializable())
	if err != nil {
		return fmt.Errorf("etcd get failed")
	}

	resources := make(map[string]*master.ResourceSpec)
	for _, kv := range resp.Kvs {
		r, err := master.Decode(kv.Value)
		if err == nil && r != nil {
			id := getID(r.AssignedNode)
			if len(id) > 0 && c.id == id {
				resources[r.Name] = r
			}
		}

	}
	c.Logger.Info("lead init load resource ", zap.Int("length", len(resources)))
	c.rlock.Lock()
	defer c.rlock.Unlock()
	c.resources = resources
	for _, resource := range resources {
		c.runTask(resource.Name)
	}
	return nil
}

func (c *Crawler) deleteTask(taskName string) {
	t, ok := Store.Hash[taskName]
	if !ok {
		c.Logger.Info("cannot predefined task", zap.String("task name", taskName))
		return
	}
	t.Closed = true
	delete(c.resources, taskName)
}

func (c *Crawler) runTask(taskName string) {
	t, ok := Store.Hash[taskName]
	if !ok {
		c.Logger.Info("cannot predefined task", zap.String("task name", taskName))
		return
	}
	t.Closed = false
	res, err := t.Rule.Root()
	if err != nil {
		c.Logger.Error("get root failed", zap.Error(err))
		return
	}
	for _, req := range res {
		req.Task = t
	}
	c.scheduler.Push(res...)

}
