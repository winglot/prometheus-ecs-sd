package discovery

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/go-kit/log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/go-kit/log/level"
	"github.com/prometheus/common/model"
	"github.com/prometheus/prometheus/discovery/targetgroup"
)

var (
	labelEcsPrefix           = model.MetaLabelPrefix + "ecs_"
	labelEcsServiceTagPrefix = labelEcsPrefix + "service_tag_"
	labelClusterName         = model.LabelName(labelEcsPrefix + "cluster_name")
	labelServiceName         = model.LabelName(labelEcsPrefix + "service_name")
)

type Option func(*discovery) error
type discovery struct {
	refreshInterval time.Duration
	client          ecsClient
	logger          log.Logger
}

type ecsClient interface {
	ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error)
	ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error)
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
	ListTagsForResource(ctx context.Context, params *ecs.ListTagsForResourceInput, optFns ...func(*ecs.Options)) (*ecs.ListTagsForResourceOutput, error)
}

func WithRefreshInterval(refreshInterval int) Option {
	return func(d *discovery) error {
		if refreshInterval <= 0 {
			return errors.New("refresh interval must be greater than 0")
		}

		d.refreshInterval = time.Second * time.Duration(refreshInterval)
		return nil
	}
}

func WithAWSECSClient(client ecsClient) Option {
	return func(d *discovery) error {
		if client == nil {
			return errors.New("aws ecs client must not be nil")
		}

		d.client = client
		return nil
	}
}

func WithLogger(logger log.Logger) Option {
	return func(d *discovery) error {
		if logger == nil {
			return errors.New("logger must not be nil")
		}
		d.logger = logger
		return nil
	}
}

func NewDiscovery(opts ...Option) (*discovery, error) {
	d := &discovery{
		refreshInterval: time.Second * 60,
		logger:          log.NewNopLogger(),
	}

	for _, opt := range opts {
		if err := opt(d); err != nil {
			return nil, err
		}
	}

	if d.client == nil {
		d.client = NewDefaultECSClient()
	}

	return d, nil
}

func NewDefaultECSClient() ecsClient {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		panic(err)
	}

	return ecs.NewFromConfig(cfg)
}

// Run implements the Discoverer interface for Prometheus SD.
// It queries the AWS ECS API for all clusters, services, and tasks and sends as target.TargetGroup to the ch channel.
func (d *discovery) Run(ctx context.Context, ch chan<- []*targetgroup.Group) {
	fmt.Println("Starting Run")
	level.Info(d.logger).Log("msg", "Starting Run", "refreshInterval", d.refreshInterval)

	ticker := time.NewTicker(d.refreshInterval)
	for c := ticker.C; ; {
		tgs, err := d.refresh(ctx)

		if err != nil {
			level.Error(d.logger).Log("msg", "Error in refresh loop", "err", err)
		} else {
			ch <- tgs
		}
		// Wait for ticker or exit when ctx is closed.
		select {
		case <-c:
			continue
		case <-ctx.Done():
			return
		}
	}
}

func (d *discovery) refresh(ctx context.Context) ([]*targetgroup.Group, error) {
	level.Info(d.logger).Log("msg", "Refreshing targets")

	clusters, err := d.client.ListClusters(ctx, &ecs.ListClustersInput{})
	if err != nil {
		return nil, err
	}
	tgs := []*targetgroup.Group{}
	for _, cluster := range clusters.ClusterArns {
		level.Info(d.logger).Log("msg", "Checking cluster", "cluster", cluster)

		clusterName := cluster[strings.LastIndex(cluster, "/")+1:]
		services, err := d.client.ListServices(ctx, &ecs.ListServicesInput{
			Cluster: aws.String(cluster),
		})

		if err != nil {
			return nil, err
		}

		for _, service := range services.ServiceArns {
			level.Info(d.logger).Log("msg", "Checking service", "service", service)

			serviceName := service[strings.LastIndex(service, "/")+1:]
			taskList, err := d.client.ListTasks(ctx, &ecs.ListTasksInput{
				Cluster:     aws.String(cluster),
				ServiceName: aws.String(service),
			})
			if err != nil {
				return nil, err
			}

			tasks, err := d.client.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: aws.String(cluster),
				Tasks:   taskList.TaskArns,
			})
			if err != nil {
				return nil, err
			}

			tags, err := d.client.ListTagsForResource(ctx, &ecs.ListTagsForResourceInput{ResourceArn: aws.String(service)})
			if err != nil {
				return nil, err
			}

			tg := &targetgroup.Group{
				Source: clusterName + "/" + serviceName,
				Labels: tagsToLabelSet(tags.Tags).Merge(model.LabelSet{
					labelClusterName: model.LabelValue(clusterName),
					labelServiceName: model.LabelValue(serviceName),
				}),
				Targets: make([]model.LabelSet, 0, len(tasks.Tasks)),
			}

			for _, task := range tasks.Tasks {
				if len(task.Containers) == 0 || len(task.Containers[0].NetworkInterfaces) == 0 {
					level.Warn(d.logger).Log("msg", "Task has no network interfaces", "task", task, "cluster", clusterName, "service", serviceName)
					continue
				}

				ip := task.Containers[0].NetworkInterfaces[0].PrivateIpv4Address
				tg.Targets = append(tg.Targets, model.LabelSet{
					model.AddressLabel: model.LabelValue(*ip),
				})
			}

			tgs = append(tgs, tg)
		}
	}

	level.Info(d.logger).Log("msg", "Refreshed targets", "targets", len(tgs))
	return tgs, nil
}

func tagsToLabelSet(tags []types.Tag) model.LabelSet {
	labels := model.LabelSet{}
	valid := regexp.MustCompile("[^a-zA-Z0-9_]")
	separator := regexp.MustCompile("([a-z])([A-Z])")
	for _, tag := range tags {
		// convert tag key to valid label name
		key := valid.ReplaceAllString(*tag.Key, "_")
		// add _ between big letters
		key = separator.ReplaceAllString(key, "${1}_${2}")
		// convert to lower case
		key = strings.ToLower(key)
		labels[model.LabelName(labelEcsServiceTagPrefix+key)] = model.LabelValue(*tag.Value)
	}
	return labels
}
