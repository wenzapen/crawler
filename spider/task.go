package spider

import (
	"sync"
)

type Property struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Cookie   string `json:"cookie"`
	WaitTime int64  `json:"wait_time"`
	Reload   bool   `json:"reload"`
	MaxDepth int64  `json:"max_depth"`
}

type TaskConfig struct {
	Name     string
	Cookie   string
	WaitTime int64
	Reload   bool
	MaxDepth int64
	Fetcher  string
	Limits   []LimitConfig
}

type LimitConfig struct {
	EventCount    int
	EventDuration int //second
	Bucket        int //size of bucket

}

type Task struct {
	Visited     map[string]bool
	VisitedLock sync.Mutex
	Rule        RuleTree
	Closed      bool
	Options
}

type Fetcher interface {
	Get(url *Request) ([]byte, error)
}

func NewTask(opts ...Option) *Task {
	options := defaultOptions
	for _, opt := range opts {
		opt(&options)
	}
	t := &Task{}
	t.Options = options
	return t
}
