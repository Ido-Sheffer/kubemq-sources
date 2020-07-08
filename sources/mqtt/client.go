package mqtt

import (
	"context"
	"fmt"
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/kubemq-hub/kubemq-source-connectors/config"
	"github.com/kubemq-hub/kubemq-source-connectors/middleware"
	"github.com/kubemq-hub/kubemq-source-connectors/pkg/logger"
	"github.com/kubemq-hub/kubemq-source-connectors/types"
	"time"
)

const (
	defaultConnectTimeout = 5 * time.Second
)

type Client struct {
	name   string
	opts   options
	client mqtt.Client
	log    *logger.Logger
	target middleware.Middleware
}

func New() *Client {
	return &Client{}
}
func (c *Client) Name() string {
	return c.name
}
func (c *Client) Init(ctx context.Context, cfg config.Metadata) error {
	c.name = cfg.Name
	c.log = logger.NewLogger(cfg.Name)
	var err error
	c.opts, err = parseOptions(cfg)
	if err != nil {
		return err
	}
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s", c.opts.host))
	opts.SetUsername(c.opts.username)
	opts.SetPassword(c.opts.password)
	opts.SetClientID(c.opts.clientId)
	opts.SetConnectTimeout(defaultConnectTimeout)
	c.client = mqtt.NewClient(opts)
	if token := c.client.Connect(); token.Wait() && token.Error() != nil {
		return fmt.Errorf("error connecting to mqtt broker, %w", token.Error())
	}
	return nil
}

func (c *Client) Start(ctx context.Context, target middleware.Middleware) error {
	if target == nil {
		return fmt.Errorf("invalid target received, cannot be nil")
	} else {
		c.target = target
	}

	c.client.Subscribe(c.opts.topic, byte(c.opts.qos), func(client mqtt.Client, message mqtt.Message) {
		go c.processIncomingMessages(ctx, message)
	})

	return nil
}

func (c *Client) processIncomingMessages(ctx context.Context, msg mqtt.Message) {
	req := types.NewRequest().SetData(msg.Payload())
	_, err := c.target.Do(ctx, req)
	if err != nil {
		c.log.Errorf("error processing mqtt message %d , %s", msg.MessageID(), err.Error())
	}
}
func (c *Client) Stop() error {
	c.client.Disconnect(250)
	return nil
}
