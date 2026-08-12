package nats

import (
	"context"
	"fmt"
	"time"

	"github.com/nassabiq/go-chi-boilerplate/internal/config"
	natsgo "github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

const (
	AppEventsStream     = "APP_EVENTS"
	AuthSubjectWildcard = "auth.>"
)

type Client struct {
	Conn      *natsgo.Conn
	JetStream jetstream.JetStream
}

func New(cfg *config.Config) (*Client, error) {
	options := []natsgo.Option{
		natsgo.Timeout(5 * time.Second),
		natsgo.PingInterval(20 * time.Second),
		natsgo.MaxPingsOutstanding(3),
		natsgo.ReconnectWait(2 * time.Second),
		natsgo.MaxReconnects(-1),
	}

	if cfg.NATS.Username != "" {
		options = append(options, natsgo.UserInfo(cfg.NATS.Username, cfg.NATS.Password))
	}

	conn, err := natsgo.Connect(cfg.NATS.URL, options...)
	if err != nil {
		return nil, fmt.Errorf("connect to nats: %w", err)
	}

	js, err := jetstream.New(conn)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create jetstream client: %w", err)
	}

	ctx, cancel := context.WithTimeout(
		context.Background(),
		5*time.Second,
	)
	defer cancel()

	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:      AppEventsStream,
		Subjects:  []string{AuthSubjectWildcard},
		Retention: jetstream.LimitsPolicy,
		Storage:   jetstream.FileStorage,
		Discard:   jetstream.DiscardOld,
		MaxAge:    72 * time.Hour,
	})

	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("create or update nats stream: %w", err)
	}

	return &Client{
		Conn:      conn,
		JetStream: js,
	}, nil
}

func (c *Client) Close() {
	if c == nil || c.Conn == nil {
		return
	}

	_ = c.Conn.Drain()
	c.Conn.Close()
}
