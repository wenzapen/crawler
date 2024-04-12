package master

import (
	"github.com/wenzapen/crawler/spider"
	"go-micro.dev/v4/registry"
	"go.uber.org/zap"
)

type options struct {
	logger      *zap.Logger
	registryURL string
	GRPCAddress string
	regisry     registry.Registry
	Seeds       []*spider.Task
}

var DefaultOptions = options{
	logger: zap.NewNop(),
}

type Option func(opts *options)

func WithLogger(logger *zap.Logger) Option {
	return func(opts *options) {
		opts.logger = logger
	}
}

func WithregistryURL(registryURL string) Option {
	return func(opts *options) {
		opts.registryURL = registryURL
	}
}

func WithGRPCAddress(GRPCAddress string) Option {
	return func(opts *options) {
		opts.GRPCAddress = GRPCAddress
	}
}

func Withregistry(registry registry.Registry) Option {
	return func(opts *options) {
		opts.regisry = registry
	}
}

func WithSeeds(seeds []*spider.Task) Option {
	return func(opts *options) {
		opts.Seeds = seeds
	}
}
