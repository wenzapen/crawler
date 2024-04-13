package master

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"time"

	grpccli "github.com/go-micro/plugins/v4/client/grpc"
	"github.com/go-micro/plugins/v4/config/encoder/toml"
	"github.com/go-micro/plugins/v4/registry/etcd"
	"github.com/go-micro/plugins/v4/server/grpc"
	"github.com/go-micro/plugins/v4/wrapper/breaker/hystrix"
	ratePlugin "github.com/go-micro/plugins/v4/wrapper/ratelimiter/ratelimit"
	"github.com/grpc-ecosystem/grpc-gateway/v2/runtime"
	"github.com/juju/ratelimit"
	"github.com/spf13/cobra"
	"github.com/wenzapen/crawler/cmd/worker"
	"github.com/wenzapen/crawler/generator"
	"github.com/wenzapen/crawler/log"
	"github.com/wenzapen/crawler/master"
	proto "github.com/wenzapen/crawler/proto/crawler"
	"github.com/wenzapen/crawler/spider"
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
	grpc2 "google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var MasterCmd = &cobra.Command{
	Use:   "master",
	Short: "run master service",
	Long:  "run master service",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		run()
	},
}

var masterID string
var cfgFile string
var HTTPListenAddress string
var GRPCListenAddress string
var PProfListenAddress string
var podIP string

func init() {
	MasterCmd.Flags().StringVar(&masterID, "id", "", "set master id")
	MasterCmd.Flags().StringVar(&cfgFile, "config", "config.toml", "set config file")
	MasterCmd.Flags().StringVar(&HTTPListenAddress, "http", ":8081", "set http listen address")
	MasterCmd.Flags().StringVar(&GRPCListenAddress, "grpc", ":9091", "set grpc listen address")
	MasterCmd.Flags().StringVar(&PProfListenAddress, "pprof", ":9981", "set pprof address")
	MasterCmd.Flags().StringVar(&podIP, "podip", "", "set podip")

}

func run() {
	go func() {
		if err := http.ListenAndServe(PProfListenAddress, nil); err != nil {
			panic(err)
		}
	}()

	var (
		err    error
		logger *zap.Logger
	)

	enc := toml.NewEncoder()
	cfg, err := config.NewConfig(config.WithReader(json.NewReader(reader.WithEncoder(enc))))
	err = cfg.Load(file.NewSource(file.WithPath(cfgFile), source.WithEncoder(enc)))

	if err != nil {
		panic(err)
	}

	// log
	logText := cfg.Get("logLevel").String("INFO")
	logLevel, err := zapcore.ParseLevel(logText)
	if err != nil {
		panic(err)
	}
	plugin := log.NewStdoutPlugin(logLevel)
	logger = log.NewLogger(plugin)
	logger.Info("log init end")

	// set zap global logger
	zap.ReplaceGlobals(logger)

	var scfg ServerConfig
	if err := cfg.Get("MasterServer").Scan(&scfg); err != nil {
		logger.Error("get master config failed", zap.Error(err))
	}
	logger.Sugar().Debugf("master config,%+v", scfg)

	reg := etcd.NewRegistry(registry.Addrs(scfg.RegistryAddress))

	var tcfg []spider.TaskConfig
	if err := config.Get("Tasks").Scan(&tcfg); err != nil {
		logger.Error("init seed tasks ", zap.Error(err))
	}
	seeds := worker.ParseTaskConfig(logger, nil, nil, tcfg)

	m, err := master.New(
		masterID,
		master.WithLogger(logger.Named("master")),
		master.WithGRPCAddress(GRPCListenAddress),
		master.WithregistryURL(scfg.RegistryAddress),
		master.Withregistry(reg),
		master.WithSeeds(seeds),
	)
	if err != nil {
		logger.Error("init master failed", zap.Error(err))
	}

	go RunHTTPServer(scfg)
	go RunGRPCServer(m, logger, reg, scfg)

}

type ServerConfig struct {
	RegistryAddress  string
	RegisterTTL      int
	RegisterInterval int
	Name             string
	ClientTimeout    int
}

func RunGRPCServer(m *master.Master, logger *zap.Logger, reg registry.Registry, cfg ServerConfig) {
	b := ratelimit.NewBucketWithRate(0.5, 1)
	if masterID == "" {
		if podIP != "" {
			id := generator.GetIDbyIP(podIP)
			masterID = strconv.Itoa(int(id))

		} else {
			masterID = fmt.Sprintf("%d", time.Now().UnixNano())
		}
	}
	zap.S().Debug("master id:", masterID)
	service := micro.NewService(
		micro.Server(grpc.NewServer(server.Id(masterID))),
		micro.Address(GRPCListenAddress),
		micro.Registry(reg),
		micro.RegisterTTL(time.Duration(cfg.RegisterTTL)*time.Second),
		micro.RegisterInterval(time.Duration(cfg.RegisterInterval)*time.Second),
		micro.WrapHandler(logWrapper(logger)),
		micro.WrapHandler(ratePlugin.NewHandlerWrapper(b, false)),
		micro.Name(cfg.Name),
		micro.Client(grpccli.NewClient()),
		micro.WrapClient(hystrix.NewClientWrapper()),
	)
	hystrix.ConfigureCommand("go.micro.server.master.CrawlerMaster.AddResource", hystrix.CommandConfig{
		Timeout:                10000,
		MaxConcurrentRequests:  100,
		RequestVolumeThreshold: 10,
		SleepWindow:            6000,
		ErrorPercentThreshold:  30,
	})
	cl := proto.NewCrawlerMasterService(cfg.Name, service.Client())
	m.SetForwardClient(cl)
	if err := service.Client().Init(client.RequestTimeout(time.Duration(cfg.ClientTimeout) * time.Second)); err != nil {
		logger.Sugar().Error("micro client init error. ", zap.String("error:", err.Error()))

		return
	}

	service.Init()
	if err := proto.RegisterCrawlerMasterHandler(service.Server(), m); err != nil {
		logger.Fatal("register handler failed", zap.Error(err))
	}

	if err := service.Run(); err != nil {
		logger.Fatal("grpc server stop", zap.Error(err))
	}
}

func RunHTTPServer(cfg ServerConfig) {
	ctx := context.Background()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	mux := runtime.NewServeMux()

	opts := []grpc2.DialOption{
		grpc2.WithTransportCredentials(insecure.NewCredentials()),
	}
	if err := proto.RegisterCrawlerMasterGwFromEndpoint(ctx, mux, GRPCListenAddress, opts); err != nil {
		zap.L().Fatal("Register backend grpc server endpoint failed", zap.Error(err))
	}
	zap.S().Debugf("start master http server listening on %v proxy to grpc server;%v", HTTPListenAddress, GRPCListenAddress)
	if err := http.ListenAndServe(HTTPListenAddress, mux); err != nil {
		zap.L().Fatal("http listenAndServe failed", zap.Error(err))
	}
}

func logWrapper(logger *zap.Logger) server.HandlerWrapper {
	return func(hf server.HandlerFunc) server.HandlerFunc {
		return func(ctx context.Context, req server.Request, rsp interface{}) error {
			logger.Info("receive request",
				zap.String("method", req.Method()),
				zap.String("Service", req.Service()),
				zap.String("Endpoint", req.Endpoint()),
				zap.Reflect("request param:", req.Body()))

			return hf(ctx, req, rsp)
		}
	}
}
