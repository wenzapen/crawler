package spider

import (
	"github.com/wenzapen/crawler/limiter"
	"go.uber.org/zap"
)

type Options struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Cookie   string `json:"cookie"`
	WaitTime int64  `json:"wait_time"`
	Reload   bool   `json:"reload"`
	MaxDepth int64  `json:"max_depth"`
	Fetcher  Fetcher
	Storage  Storage
	Limit    limiter.RateLimiter
	logger   *zap.Logger
}

var defaultOptions = Options{
	logger:   zap.NewNop(),
	WaitTime: 5,
	Reload:   false,
	MaxDepth: 5,
}

type Option func(opts *Options)

func WithLogger(logger *zap.Logger) Option {
	return func(opts *Options) {
		opts.logger = logger
	}
}

func WithName(name string) Option {
	return func(opts *Options) {
		opts.Name = name
	}
}

func WithURL(url string) Option {
	return func(opts *Options) {
		opts.URL = url
	}
}

func WithCookie(cookie string) Option {
	return func(opts *Options) {
		opts.Cookie = cookie
	}
}

func WithWaitTime(waittime int64) Option {
	return func(opts *Options) {
		opts.WaitTime = waittime
	}
}

func WithReload(reload bool) Option {
	return func(opts *Options) {
		opts.Reload = reload
	}
}

func WithMaxDepth(maxdepth int64) Option {
	return func(opts *Options) {
		opts.MaxDepth = maxdepth
	}
}

func WithFetcher(fetcher Fetcher) Option {
	return func(opts *Options) {
		opts.Fetcher = fetcher
	}
}

func WithStorage(storage Storage) Option {
	return func(opts *Options) {
		opts.Storage = storage
	}
}
