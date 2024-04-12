package worker

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-micro/plugins/v4/config/encoder/toml"
	"github.com/go-micro/plugins/v4/registry/etcd"
	"github.com/go-micro/plugins/v4/server/grpc"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/spf13/cobra"
	"github.com/wenzapen/crawler/collect"
	"github.com/wenzapen/crawler/engine"
	"github.com/wenzapen/crawler/generator"
	"github.com/wenzapen/crawler/limiter"
	"github.com/wenzapen/crawler/log"
	"github.com/wenzapen/crawler/proxy"
	"github.com/wenzapen/crawler/spider"
	"github.com/wenzapen/crawler/storage/sqlstorage"
	"go-micro.dev/v4"
	"go-micro.dev/v4/client"
	"go-micro.dev/v4/config"
	"go-micro.dev/v4/config/reader"
	"go-micro.dev/v4/config/reader/json"
	"go-micro.dev/v4/config/source"
	"go-micro.dev/v4/config/source/file"
	"go-micro.dev/v4/registry"
	"go-micro.dev/v4/server"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"golang.org/x/time/rate"
	"google.golang.org/grpc"
	grpc2 "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var ServiceName string = "go.micro.server.worker"

var WorkerCmd = &cobra.Command{
	Use:   "worker",
	Short: "run worker service",
	Long:  "run worker service",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		Run()
	},
}

func init() {
	WorkerCmd.Flags().StringVar(&workerID, "id", "", "set worker id")
	WorkerCmd.Flags().StringVar(&HTTPListenAddress, "http", "8080", "set http listen address")
	WorkerCmd.Flags().StringVar(&GPRCListenAddress, "grpc", "9090", "set grpc listen address")
	WorkerCmd.Flags().StringVar(&PProfListenAddress, "pprof", "9981", "set pprof address")
	WorkerCmd.Flags().StringVar(&podIP, "podip", "", "set pod ip address")
	WorkerCmd.Flags().BoolVar(&cluster, "cluster", true, "run mode")
}

var cluster bool

var workerID string
var HTTPListenAddress string
var GPRCListenAddress string
var PProfListenAddress string
var podIP string

func Run() {
	go func() {
		if err := http.ListenAndServe(PProfListenAddress, nil); err != nil {
			panic(err)
		}
	}()

	var (
		err     error
		logger  *zap.Logger
		p       proxy.ProxyFunc
		storage spider.Storage
	)

	enc := toml.NewEncoder()
	cfg, err := config.NewConfig(config.WithReader(json.NewReader(reader.WithEncoder(enc))))
	err = cfg.Load(file.NewSource(file.WithPath("config.toml"), source.WithEncoder(enc)))
	if err != nil {
		panic(err)
	}

	logText := cfg.Get("logLevel").String("INFO")
	logLevel, err := zapcore.ParseLevel(logText)
	if err != nil {
		panic(err)
	}
	plugin := log.NewStdoutPlugin(logLevel)
	logger = log.NewLogger(plugin)
	logger.Info("log init end")
	zap.ReplaceGlobals(logger)

	proxyURLs := cfg.Get("fetcher", "proxy").StringSlice([]string{})
	timeout := cfg.Get("fetcher", "timeout").Int(5000)
	logger.Sugar().Info("proxy list: ", proxyURLs, " timeout: ", timeout)
	if p, err = proxy.RoundRobinSwitcher(proxyURLs...); err != nil {
		logger.Error("RoundRobinProxySwitcher", zap.Error(err))
	}
	var f spider.Fetcher = &collect.BrowserFetch{
		Timeout: time.Duration(timeout) * time.Millisecond,
		Logger:  logger,
		Proxy:   p,
	}

	sqlURL := cfg.Get("storage", "sqlURL").String("")
	if storage, err = sqlstorage.New(
		sqlstorage.WithBatchCount(2),
		sqlstorage.WithLogger(logger),
		sqlstorage.WithSQLURL(sqlURL),
	); err != nil {
		logger.Error("create sql storage failed", zap.Error(err))
		return
	}

	var tcfg []spider.TaskConfig
	if err = cfg.Get("Tasks").Scan(&tcfg); err != nil {
		logger.Error("init seed tasks failed", zap.Error(err))
	}
	seeds := ParseTaskConfig(logger, f, storage, tcfg)
	var sconfig ServerConfig
	if err := cfg.Get("GRPCServer").Scan(&sconfig); err != nil {
		logger.Error("get GRPC Server config failed", zap.Error(err))
	}
	logger.Sugar().Debugf("grpc server config,%+v", sconfig)

	s, err := engine.NewEngine(
		engine.WithFetcher(f),
		engine.WithLogger(logger),
		engine.WithWorkCount(5),
		engine.WithSeeds(seeds),
		engine.WithregistryURL(sconfig.RegistryAddress),
		engine.WithScheduler(engine.NewSchedule()),
		engine.WithStorage(storage),
	)

	if err != nil {
		panic(err)
	}

	if workerID == "" {
		if podIP != "" {
			ip := generator.GetIDbyIP(podIP)
			workerID = strconv.Itoa(int(ip))
		} else {
			workerID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
	}

	id := sconfig.Name + "-" + workerID
	zap.S().Debug("worker id:", id)

	// worker start
	go s.Run(id, cluster)

	// start http proxy to GRPC
	go RunHTTPServer(sconfig)

	// start grpc server
	RunGRPCServer(logger, sconfig)
}

type ServerConfig struct {
	RegistryAddress  string
	RegisterTTL      int
	RegisterInterval int
	Name             string
	ClientTimeout    int
}

func RunGRPCServer(logger *zap.Logger, cfg ServerConfig) {
	reg := etcd.NewRegistry(registry.Addrs(cfg.RegistryAddress))
	service := micro.NewService(
		micro.Server(grpc.NewServer(
			server.Id(workerID),
		)),
		micro.Address(GPRCListenAddress),
		micro.Registry(reg),
		micro.RegisterTTL(time.Duration(cfg.RegisterTTL)*time.Second),
		micro.RegisterInterval(time.Duration(cfg.RegisterInterval)*time.Second),
		micro.WrapHandler(logWrapper(logger)),
		micro.Name(cfg.Name),
	)

	if err := service.Client().init(client.RequestTimeout(time.Duration(cfg.ClientTimeOut) * time.Second)); err != nil {
		logger.Sugar().Error("micro client init error. ", zap.String("error:", err.Error()))

		return
	}

	service.Init()
	if err := greeter.RegisterGreeterHandler(service.Server(), new(Greeter)); err != nil {
		logger.Fatal("register handler failed", zap.Error(err))
	}

	if err := service.Run(); err != nil {
		logger.Fatal("grpc server stop", zap.Error(err))
	}
}

type Greeter struct {
}

func (g *Greeter) Hello(ctx context.Context, req *greeter.Request, resp *greeter.Response) error {
	resp.Greeting = "hello " + req.Name
	return nil
}

func RunHTTPServer(cfg ServerConfig) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	mux := runtime.NewServeMux()
	opts := []grpc2.DialOption{
		grpc2.WithTransportCredentials(insecure.NewCredentials()),
	}
	if err := greeter.RegisterGreeterGwFromEndpoint(ctx, mux, GRPCListenAddress, opts); err != nil {
		zap.L().Fatal("Register backend grpc server endpoint failed", zap.Error(err))
	}
	zap.S().Debugf("start http server listening on %v proxy to grpc server;%v", HTTPListenAddress, GRPCListenAddress)
	if err := http.ListenAndServe(HTTPListenAddress, mux); err != nil {
		zap.L().Fatal("http listenAndServe failed", zap.Error(err))
	}
}

func logWrapper(logger *zap.Logger) server.HandlerWrapper {
	return func(hf server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			logger.Info("receive request",
				zap.String("method", req.Method()),
				zap.String("service", req.Service()),
				zap.Any("request param:", req.Body()))
			err := hf(ctx, req, rsp)
			return err
		}
	}
}

func ParseTaskConfig(logger *zap.Logger, f spider.Fetcher, s spider.Storage, cfgs []spider.TaskConfig) []*spider.Task {
	tasks := make([]*spider.Task, 0, 1000)
	for _, cfg := range cfgs {
		t := spider.NewTask(
			spider.WithName(cfg.Name),
			spider.WithCookie(cfg.Cookie),
			spider.WithReload(cfg.Reload),
			spider.WithLogger(logger),
			spider.WithStorage(s),
		)
		if cfg.WaitTime > 0 {
			spider.WithWaitTime(cfg.WaitTime)
		}
		if cfg.MaxDepth > 0 {
			spider.WithMaxDepth(cfg.MaxDepth)
		}
		var limits []limiter.RateLimiter
		if len(cfg.Limits) > 0 {
			for _, lcfg := range cfg.Limits {
				l := rate.NewLimiter(limiter.Per(lcfg.EventCount, time.Duration(lcfg.EventDuration)*time.Second), lcfg.Bucket)
				limits = append(limits, l)
			}
			multiLimiter := limiter.Multi(limits...)
			t.Limit = multiLimiter
		}
		switch cfg.Fetcher {
		case "browser":
			t.Fetcher = f
		}
		tasks = append(tasks, t)

	}
	return tasks
}
