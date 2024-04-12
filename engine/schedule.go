package engine

import (
	"sync"

	"github.com/wenzapen/crawler/parse/doubangroup"
	"github.com/wenzapen/crawler/spider"
	"go.uber.org/zap"
)

var Store = &CrawlerStore{
	list: []*spider.Task{},
	Hash: make(map[string]*spider.Task),
}

func init() {
	Store.Add(doubangroup.DoubanGroupTask)
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
	out         chan spider.ParseResult
	Visited     map[string]bool
	VisitedLock sync.Mutex

	failures    map[string]*spider.Request
	failureLock sync.Mutex
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

func (e *Crawler) Run() {
	go e.Schedule()
	for i := 0; i < e.WorkCount; i++ {
		go e.CreateWorker()
	}
	e.HandleResult()
}

func (e *Crawler) Schedule() {
	reqs := []*spider.Request{}
	for _, seed := range e.Seeds {
		seed.RootReq.Task = seed
		seed.RootReq.Url = seed.Url
		reqs = append(reqs, seed.RootReq)
	}
	go e.scheduler.Schedule()
	go e.scheduler.Push(reqs...)
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
