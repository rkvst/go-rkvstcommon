package azbus

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	azadmin "github.com/Azure/azure-sdk-for-go/sdk/messaging/azservicebus/admin"
)

var (
	// lots of docs by MS on how these limits are set to various values here
	// https://docs.microsoft.com/en-us/azure/service-bus-messaging/service-bus-quotas but
	defaultMaxMessageSize = int64(256 * 1024)
	ErrMessageOversized   = errors.New("message is too large")
)

// AZAdminClient provides access to the administrative client for the message
// bus. Services that self manage subscriptions are the exceptional case and
// co-ordination with devops is required before using this mechanism.
type AZAdminClient struct {
	ConnectionString string
	log              Logger
	admin            *azadmin.Client
}

func NewAZAdminClient(log Logger, connectionString string) AZAdminClient {
	return AZAdminClient{
		ConnectionString: connectionString,
		log:              log,
	}
}

// Open - connects and returns the azure admin Client interface that allows creation of topics etc.
// Note that creation is cached
func (c *AZAdminClient) Open() (*azadmin.Client, error) {

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

func (c *AZAdminClient) GetQueueMaxMessageSize(queueName string) (int64, error) {
	admin, err := c.Open()
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

func (c *AZAdminClient) GetTopicMaxMessageSize(topicName string) (int64, error) {
	admin, err := c.Open()
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

// EnsuresubscriptionRule ensures the named rule is set on the subscription and
// creates it from the supplied filter if not. Note: When the ruleName exists,
// we do not attempt check the supplied filter matches the existing filter.
func (c *AZAdminClient) EnsureSubscriptionRule(
	ctx context.Context,
	topicName, subscriptionName string,
	ruleName string,
	ruleString string,
) error {

	admin, err := c.Open()
	if err != nil {
		return err
	}
	// The default rule matches everything. If its not removed all the other
	// filters are effectively ignored. Removal is idempotent. So we always
	// remove it.
	if _, err = admin.DeleteRule(ctx, topicName, subscriptionName, "$Default", nil); err != nil {
		var respError *azcore.ResponseError
		if !errors.As(err, &respError) {
			c.log.Infof(
				"DeleteRule failed for topicname=%s subname=%s, rulename=%s: %v",
				topicName,
				subscriptionName,
				"$Default",
				err.Error(),
			)
			return err
		}
		if respError.StatusCode != http.StatusNotFound {
			c.log.Infof(
				"DeleteRule failed for topicname=%s subname=%s, rulename=%s: %v",
				topicName,
				subscriptionName,
				"$Default",
				err.Error(),
			)
			return err
		}
	}
	c.log.Debugf(
		"DeleteRule no longer exists for topicname=%s subname=%s, rulename=%s",
		topicName,
		subscriptionName,
		"$Default",
	)

	// Attempt to get the rule - strangely an error is not generated for a 404
	// Found this by reading the unittests...
	response, err := admin.GetRule(ctx, topicName, subscriptionName, ruleName, nil)
	if err != nil {
		c.log.Infof(
			"GetRule failed for topicname=%s subname=%s, rulename=%s: %v",
			topicName,
			subscriptionName,
			ruleName,
			err.Error(),
		)
		return err
	}

	// Rule does not exist so create it
	if response == nil {
		c.log.Debugf(
			"Rule does not exist for topicname=%s subname=%s, rulename=%s",
			topicName,
			subscriptionName,
			ruleName,
		)
		_, err = admin.CreateRule(
			ctx,
			topicName,
			subscriptionName,
			&azadmin.CreateRuleOptions{
				Name: &ruleName,
				Filter: &azadmin.SQLFilter{
					Expression: ruleString,
				},
			},
		)
		if err != nil {
			c.log.Infof(
				"CreateRule failed for topicname=%s subname=%s, rulename=%s: %v",
				topicName,
				subscriptionName,
				ruleName,
				err.Error(),
			)
			return err
		}
		return nil
	}

	c.log.Debugf(
		"UpdateRule for topicname=%s subname=%s, rulename=%s, ruleString=%q",
		topicName,
		subscriptionName,
		ruleName,
		ruleString,
	)
	_, err = admin.UpdateRule(
		ctx,
		topicName,
		subscriptionName,
		azadmin.RuleProperties{
			Name: ruleName,
			Filter: &azadmin.SQLFilter{
				Expression: ruleString,
			},
		},
	)
	if err != nil {
		c.log.Infof(
			"UpdateRule failed for topicname=%s subname=%s, rulename=%s, ruleString=%q: %v",
			topicName,
			subscriptionName,
			ruleName,
			ruleString,
			err.Error(),
		)
		return err
	}

	c.log.Debugf(
		"Rule exists for topicname=%s subname=%s, rulename=%s, ruleString=%q",
		topicName,
		subscriptionName,
		ruleName,
		ruleString,
	)
	return nil
}
