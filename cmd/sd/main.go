package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/prometheus/client_golang/prometheus"
	promsd "github.com/prometheus/prometheus/discovery"
	"github.com/winglot/prometheus-ecs-sd/internal/adapter"
	"github.com/winglot/prometheus-ecs-sd/internal/discovery"
)

var (
	refreshInterval int
	outputFile      string
)

func main() {
	flag.IntVar(&refreshInterval, "target.refresh", 60, "The refresh interval (in seconds).")
	flag.StringVar(&outputFile, "output.file", "ecs_sd.json", "Output file for file_sd compatible file.")
	flag.Parse()

	logger := log.NewSyncLogger(log.NewLogfmtLogger(os.Stdout))
	disc, err := discovery.NewDiscovery(
		discovery.WithLogger(logger),
		discovery.WithRefreshInterval(refreshInterval),
		discovery.WithAWSECSClient(
			discovery.NewECSCacheClient(logger, discovery.NewDefaultECSClient()),
		),
	)
	if err != nil {
		fmt.Println("err: ", err)
	}
	ctx := context.Background()
	sdAdapter := adapter.NewAdapter(
		ctx,
		outputFile,
		"prometheus-ecs-sd",
		disc,
		logger,
		map[string]promsd.DiscovererMetrics{},
		prometheus.DefaultRegisterer,
	)
	sdAdapter.Run()
	<-ctx.Done()
}
