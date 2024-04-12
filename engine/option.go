package engine

import (
	"github.com/wenzapen/crawler/spider"
	"go.uber.org/zap"
)

type Option func(opts *options)

type options struct {
	WorkCount  int
	Fetcher    spider.Fetcher
	Storage    spider.Storage
	Logger     *zap.Logger
	Seeds      []*spider.Task
	resitryURL string
	scheduler  Scheduler
}

var DefaultOptions = options{
	Logger: zap.NewNop(),
}

func WithLogger(logger *zap.Logger) Option {
	return func(opts *options) {
		opts.Logger = logger
	}
}

func WithStorage(s spider.Storage) Option {
	return func(opts *options) {
		opts.Storage = s
	}
}

func WithregistryURL(url string) Option {
	return func(opts *options) {
		opts.resitryURL = url
	}
}

func WithWorkCount(c int) Option {
	return func(opts *options) {
		opts.WorkCount = c
	}
}

func WithFetcher(fetcher spider.Fetcher) Option {
	return func(opts *options) {
		opts.Fetcher = fetcher
	}
}

func WithSeeds(seeds []*spider.Task) Option {
	return func(opts *options) {
		opts.Seeds = seeds
	}
}

func WithScheduler(scheduler Scheduler) Option {
	return func(opt *options) {
		opt.scheduler = scheduler
	}
}
