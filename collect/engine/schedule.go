package engine

import "github.com/wenzapen/crawler/collect"

type ScheduleEngine struct {
	requestChan chan *collect.Request
	workerChan  chan *collect.Request
	WorkCount   int
	Fetcher     collect.Fetcher
	out         chan collect.ParseResult
	Seeds       []*collect.Request
}

func (s *ScheduleEngine) Run() {
	s.requestChan = make(chan *collect.Request)
	s.workerChan = make(chan *collect.Request)
	s.out = make(chan collect.ParseResult)

	go s.Schedule()
	for i := 0; i < s.WorkCount; i++ {
		go s.CreateWorker()
	}
	s.HandleResult()

}

func (s *ScheduleEngine) Schedule() {

}

func (s *ScheduleEngine) HandleResult() {

}

func (s *ScheduleEngine) CreateWorker() {

}
