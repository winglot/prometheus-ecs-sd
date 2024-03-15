package discovery

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/ecs"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/go-kit/log"
	"github.com/go-kit/log/level"
	gocache "github.com/winglot/prometheus-ecs-sd/internal/cache"
)

type ecsClientCache struct {
	client ecsClient
	logger log.Logger
	cache  *gocache.Cache
}

func NewECSCacheClient(logger log.Logger, client ecsClient) ecsClient {
	return &ecsClientCache{
		client: client,
		logger: logger,
		cache: gocache.New(
			gocache.WithDefaultExpiration(15*time.Minute),
			gocache.WithJanitor(30*time.Minute),
			gocache.WithGetReturnStale(),
		),
	}
}

func (c *ecsClientCache) ListClusters(ctx context.Context, params *ecs.ListClustersInput, optFns ...func(*ecs.Options)) (*ecs.ListClustersOutput, error) {
	cached, ok := c.cache.Get("ListClusters")
	if ok {
		level.Info(c.logger).Log("msg", "ListClusters cache hit")
		return cached.(*ecs.ListClustersOutput), nil
	}

	v, err := c.client.ListClusters(ctx, params, optFns...)
	if err != nil {
		if cached != nil {
			level.Warn(c.logger).Log("msg", "ListClusters failed, stale cache response found and used", "err", err)
			return cached.(*ecs.ListClustersOutput), nil
		}
	}
	c.cache.Set("ListClusters", v, time.Hour)
	return v, err
}

func (c *ecsClientCache) ListServices(ctx context.Context, params *ecs.ListServicesInput, optFns ...func(*ecs.Options)) (*ecs.ListServicesOutput, error) {
	cluster := *params.Cluster
	cached, ok := c.cache.Get("ListServices-" + cluster)

	if ok {
		level.Info(c.logger).Log("msg", "ListServices cache hit", "cluster", cluster)
		return cached.(*ecs.ListServicesOutput), nil
	}

	v, err := c.client.ListServices(ctx, params, optFns...)
	if err != nil {
		if cached != nil {
			level.Warn(c.logger).Log("msg", "ListServices failed, stale cache response found and used", "err", err)
			return cached.(*ecs.ListServicesOutput), nil
		}

		return v, err
	}

	c.cache.Set("ListServices-"+cluster, v, 5*time.Minute)
	return v, nil
}

func (c *ecsClientCache) ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error) {
	return c.client.ListTasks(ctx, params, optFns...)
}

func (c *ecsClientCache) DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error) {
	cachedTasks := []types.Task{}
	uncachedTasks := []string{}
	if params != nil && params.Tasks != nil {
		for _, task := range params.Tasks {
			if v, ok := c.cache.Get("DescribeTasks-" + task); ok {
				level.Info(c.logger).Log("msg", "DescribeTasks cache hit", "task", task)
				cachedTasks = append(cachedTasks, *v.(*types.Task))
				continue
			}
			uncachedTasks = append(uncachedTasks, task)
		}
	}

	if len(uncachedTasks) == 0 {
		return &ecs.DescribeTasksOutput{Tasks: cachedTasks}, nil
	}

	if params == nil {
		params = &ecs.DescribeTasksInput{}
	}

	params.Tasks = uncachedTasks
	response, err := c.client.DescribeTasks(ctx, params, optFns...)
	if err != nil {
		return response, err
	}
	for _, task := range response.Tasks {
		c.cache.SetDefault("DescribeTasks-"+*task.TaskArn, &task)
	}

	response.Tasks = append(response.Tasks, cachedTasks...)
	return response, nil
}

func (c *ecsClientCache) ListTagsForResource(ctx context.Context, params *ecs.ListTagsForResourceInput, optFns ...func(*ecs.Options)) (*ecs.ListTagsForResourceOutput, error) {
	arn := *params.ResourceArn
	cached, ok := c.cache.Get("ListTagsForResource-" + arn)
	if ok {
		level.Info(c.logger).Log("msg", "ListTagsForResource cache hit", "arn", arn)
		return cached.(*ecs.ListTagsForResourceOutput), nil
	}

	v, err := c.client.ListTagsForResource(ctx, params, optFns...)
	if err != nil {
		if cached != nil {
			level.Warn(c.logger).Log("msg", "ListTagsForResource failed, stale cache response found and used", "err", err)
			return cached.(*ecs.ListTagsForResourceOutput), nil
		}
		return v, err
	}

	c.cache.SetDefault("ListTagsForResource-"+arn, v)
	return v, nil
}
