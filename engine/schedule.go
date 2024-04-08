package engine

import (
	"github.com/wenzapen/crawler/collect"
	"go.uber.org/zap"
)

type Schedule struct {
	requestChan chan *collect.Request
	workerChan  chan *collect.Request
	out         chan collect.ParseResult
	options
}

func NewSchedule(opts ...Option) *Schedule {
	s := Schedule{}
	options := DefaultOptions
	for _, opt := range opts {
		opt(&options)
	}
	s.options = options
	return &s
}

func (s *Schedule) Run() {
	s.requestChan = make(chan *collect.Request)
	s.workerChan = make(chan *collect.Request)
	s.out = make(chan collect.ParseResult)

	go s.Schedule()
	for i := 0; i < s.WorkCount; i++ {
		go s.CreateWorker()
	}
	s.HandleResult()

}

func (s *Schedule) Schedule() {

	var reqQueue = s.Seeds
	for {
		var req *collect.Request
		var workerCh chan *collect.Request
		if len(reqQueue) > 0 {
			req = reqQueue[0]
			reqQueue = reqQueue[1:]
			workerCh = s.workerChan
		}
		select {
		case workerCh <- req:
		case r := <-s.requestChan:
			reqQueue = append(reqQueue, r)

		}
	}
}

func (s *Schedule) HandleResult() {
	for {
		result := <-s.out
		for _, req := range result.Requests {
			s.requestChan <- req
		}
		for _, item := range result.Items {
			s.Logger.Sugar().Info("get result ", item)
		}

	}
}

func (s *Schedule) CreateWorker() {
	for {
		req := <-s.workerChan
		body, err := s.Fetcher.Get(req)
		if err != nil {
			s.Logger.Error("can't fetch ", zap.Error(err))
			continue
		}
		result := req.ParseFunction(body, req)
		s.out <- result
	}
}
