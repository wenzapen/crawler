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
	Limit    []LimitConfig
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
	Options
}

type Fetcher interface {
	Get(url *Request) ([]byte, error)
}
