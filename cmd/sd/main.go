package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	"github.com/prometheus/client_golang/prometheus"
	promsd "github.com/prometheus/prometheus/discovery"
	"github.com/winglot/prometheus-ecs-sd/internal/adapter"
	"github.com/winglot/prometheus-ecs-sd/internal/discovery"
)

var (
	refreshInterval int
	outputFile      string
	logLevel        string
)

func main() {
	flag.IntVar(&refreshInterval, "target.refresh", 60, "The refresh interval (in seconds).")
	flag.StringVar(&outputFile, "output.file", "ecs_sd.json", "Output file for file_sd compatible file.")
	flag.StringVar(&logLevel, "log.level", "warn", "Set loging verbosity (debug, info, warn, error).")
	flag.Parse()

	logVerbosity, err := level.Parse(logLevel)
	if err != nil {
		fmt.Printf("error parsing log level: %v", err)
		os.Exit(1)
	}

	logger := level.NewFilter(
		log.NewSyncLogger(log.NewLogfmtLogger(os.Stdout)),
		level.Allow(logVerbosity),
	)

	disc, err := discovery.NewDiscovery(
		discovery.WithLogger(logger),
		discovery.WithRefreshInterval(refreshInterval),
		discovery.WithAWSECSClient(
			discovery.NewECSCacheClient(logger, discovery.NewDefaultECSClient()),
		),
	)
	if err != nil {
		fmt.Printf("error creating discovery: %v", err)
		os.Exit(1)
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
