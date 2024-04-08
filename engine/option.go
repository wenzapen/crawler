package engine

import (
	"github.com/wenzapen/crawler/collect"
	"go.uber.org/zap"
)

type Option func(opts *options)

type options struct {
	WorkCount int
	Fetcher   collect.Fetcher
	Logger    *zap.Logger
	Seeds     []*collect.Request
}

var DefaultOptions = options{
	Logger: zap.NewNop(),
}

func WithLogger(logger *zap.Logger) Option {
	return func(opts *options) {
		opts.Logger = logger
	}
}

func WithWorkCount(c int) Option {
	return func(opts *options) {
		opts.WorkCount = c
	}
}

func WithFetcher(fetcher collect.Fetcher) Option {
	return func(opts *options) {
		opts.Fetcher = fetcher
	}
}

func WithSeeds(seeds []*collect.Request) Option {
	return func(opts *options) {
		opts.Seeds = seeds
	}
}
