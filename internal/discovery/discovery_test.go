package discovery

import (
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs/types"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/assert"
)

func Test_TagsToLabelSet(t *testing.T) {
	tags := []types.Tag{
		{
			Key:   aws.String("key1"),
			Value: aws.String("value1"),
		},
		{
			Key:   aws.String("prometheus.io/scrape"),
			Value: aws.String("true"),
		},
		{
			Key:   aws.String("prometheus.io/port"),
			Value: aws.String("8080"),
		},
	}
	expectedLabels := model.LabelSet{
		model.LabelName("__meta_ecs_service_tag_key1"):                 model.LabelValue("value1"),
		model.LabelName("__meta_ecs_service_tag_prometheus_io_scrape"): model.LabelValue("true"),
		model.LabelName("__meta_ecs_service_tag_prometheus_io_port"):   model.LabelValue("8080"),
	}

	labels := tagsToLabelSet(tags)
	assert.Equal(t, expectedLabels, labels)
}
