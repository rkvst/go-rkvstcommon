package azbus

import (
	"context"
	"errors"
	"fmt"

	azadmin "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

var (
	// lots of docs by MS on how these limits are set to various values here
	// https://docs.microsoft.com/en-us/azure/service-bus-messaging/service-bus-quotas
	defaultMaxMessageSize = int64(256 * 1024)
	ErrMessageOversized   = errors.New("message is too large")
)

type azAdminClient struct {
	ConnectionString string
	log              Logger
	admin            *azadmin.Client
}

func newazAdminClient(log Logger, connectionString string) azAdminClient {
	return azAdminClient{
		ConnectionString: connectionString,
		log:              log,
	}
}

// open - connects and returns the azure admin Client interface that allows creation of topics etc.
// Note that creation is cached
func (c *azAdminClient) open() (*azadmin.Client, error) {

	if c.admin != nil {
		return c.admin, nil
	}

	if c.ConnectionString == "" {
		return nil, fmt.Errorf("failed to create admin client: config must provide either an account name or a connection string")
	}

	c.log.Debugf("Get new Admin client using ConnectionString")
	admin, err := azadmin.NewClientFromConnectionString(
		c.ConnectionString,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("failed creating new admin client: %w", NewAzbusError(err))
	}
	c.admin = admin
	return c.admin, nil
}

func (c *azAdminClient) getQueueMaxMessageSize(queueName string) (int64, error) {
	admin, err := c.open()
	if err != nil {
		return 0, err
	}
	q, err := admin.GetQueue(context.Background(), queueName, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get queue properties: %w", NewAzbusError(err))
	}
	c.log.DebugR("queue properties", q)
	if q.MaxMessageSizeInKilobytes != nil {
		n := *q.MaxMessageSizeInKilobytes
		return n * 1024, nil
	}
	// For non-Premium accounts the default is 256KiB and is not returned by GetQueue
	return defaultMaxMessageSize, nil
}

func (c *azAdminClient) getTopicMaxMessageSize(topicName string) (int64, error) {
	admin, err := c.open()
	if err != nil {
		return 0, err
	}
	t, err := admin.GetTopic(context.Background(), topicName, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to get topic properties: %w", NewAzbusError(err))
	}
	c.log.DebugR("topic properties", t)
	if t.MaxMessageSizeInKilobytes != nil {
		n := *t.MaxMessageSizeInKilobytes
		return n * 1024, nil
	}
	// For non-Premium accounts the default is 256KiB and is not returned by GetQueue
	return defaultMaxMessageSize, nil
}
