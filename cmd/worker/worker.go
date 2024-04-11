package worker

import (
	"net/http"

	"github.com/go-micro/plugins/v4/config/encoder/toml"
	"github.com/spf13/cobra"
	"github.com/wenzapen/crawler/proxy"
	"github.com/wenzapen/crawler/spider"
	"go-micro.dev/v4/config"
	"go-micro.dev/v4/config/reader"
	"go-micro.dev/v4/config/reader/json"
	"go-micro.dev/v4/config/source"
	"go-micro.dev/v4/config/source/file"
	"go.uber.org/zap"
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
}

type ServerConfig struct {
	RegistryAddress  string
	RegisterTTL      int
	RegisterInterval int
	Name             string
	ClientTimeout    int
}

func RunGPRCServer(logger *zap.Logger, cfg ServerConfig) {

}
