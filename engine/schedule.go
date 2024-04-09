package engine

import (
	"sync"

	"github.com/wenzapen/crawler/collect"
	"go.uber.org/zap"
)

type Crawler struct {
	out         chan collect.ParseResult
	Visited     map[string]bool
	VisitedLock sync.Mutex
	options
}

type Scheduler interface {
	Schedule()
	Push(...*collect.Request)
	Pull() *collect.Request
}

type Schedule struct {
	requestChan chan *collect.Request
	workerChan  chan *collect.Request
	reqQueue    []*collect.Request
	Logger      *zap.Logger
}

func NewEngine(opts ...Option) *Crawler {
	options := DefaultOptions
	for _, opt := range opts {
		opt(&options)
	}

	e := &Crawler{}
	e.options = options
	e.Visited = make(map[string]bool, 100)
	e.out = make(chan collect.ParseResult)

	return e
}

func NewSchedule() *Schedule {
	s := Schedule{}
	s.requestChan = make(chan *collect.Request)
	s.workerChan = make(chan *collect.Request)
	return &s
}

func (s *Schedule) Push(reqs ...*collect.Request) {
	for _, req := range reqs {
		s.requestChan <- req
	}
}

func (s *Schedule) Pull() *collect.Request {
	r := <-s.workerChan
	return r
}

func (s *Schedule) Output() *collect.Request {
	r := <-s.workerChan
	return r
}

func (s *Schedule) Schedule() {

	for {
		var req *collect.Request
		var workerCh chan *collect.Request
		if len(s.reqQueue) > 0 {
			req = s.reqQueue[0]
			s.reqQueue = s.reqQueue[1:]
			workerCh = s.workerChan
		}
		select {
		case workerCh <- req:
		case r := <-s.requestChan:
			s.reqQueue = append(s.reqQueue, r)

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
	reqs := []*collect.Request{}
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
			e.Logger.Debug("request has visited", zap.String("url:", req.Url))
			continue
		}
		e.StoreVisited(req)
		body, err := req.Task.Fetcher.Get(req)
		if err != nil {
			e.Logger.Error("can't fetch ", zap.Error(err), zap.String("url:", req.Url))
			continue
		}
		result := req.ParseFunction(body, req)
		if len(result.Requests) > 0 {
			go e.scheduler.Push(result.Requests...)
		}
		e.out <- result
	}
}

func (e *Crawler) HasVisited(r *collect.Request) bool {
	e.VisitedLock.Lock()
	defer e.VisitedLock.Unlock()
	unique := r.Unique()
	return e.Visited[unique]
}

func (e *Crawler) StoreVisited(reqs ...*collect.Request) {
	e.VisitedLock.Lock()
	defer e.VisitedLock.Unlock()
	for _, r := range reqs {
		unique := r.Unique()
		e.Visited[unique] = true
	}
}
