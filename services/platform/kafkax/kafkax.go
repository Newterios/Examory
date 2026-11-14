// Package kafkax wraps franz-go for the platform: an idempotent producer
// with acks=all and a consumer-group client builder.
package kafkax

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

// Message is one record to publish.
type Message struct {
	Topic   string
	Key     string
	Value   []byte
	Headers map[string]string
}

// Publisher publishes messages; errs[i] is the outcome of msgs[i].
// Records sharing a key are written in slice order.
type Publisher interface {
	Publish(ctx context.Context, msgs []Message) []error
}

// Config configures Kafka clients.
type Config struct {
	Brokers  []string
	ClientID string
	// DeliveryTimeout bounds how long a record may be retried before it is
	// failed back to the caller (default 15s).
	DeliveryTimeout time.Duration
	// AllowAutoTopicCreation lets the client create missing topics (dev only).
	AllowAutoTopicCreation bool
}

func (c Config) opts() []kgo.Opt {
	if c.DeliveryTimeout <= 0 {
		c.DeliveryTimeout = 15 * time.Second
	}
	o := []kgo.Opt{
		kgo.SeedBrokers(c.Brokers...),
		kgo.ClientID(c.ClientID),
		kgo.DialTimeout(5 * time.Second),
		kgo.RequestTimeoutOverhead(5 * time.Second),
		kgo.RequiredAcks(kgo.AllISRAcks()), // required by the idempotent producer
		kgo.ProducerLinger(5 * time.Millisecond),
		kgo.RecordDeliveryTimeout(c.DeliveryTimeout),
		kgo.ProduceRequestTimeout(10 * time.Second),
		kgo.ProducerBatchCompression(kgo.NoCompression()),
	}
	if c.AllowAutoTopicCreation {
		o = append(o, kgo.AllowAutoTopicCreation())
	}
	return o
}

// Producer is an idempotent Kafka producer.
type Producer struct {
	client *kgo.Client
}

// NewProducer creates a producer. Connection is lazy.
func NewProducer(cfg Config) (*Producer, error) {
	if len(cfg.Brokers) == 0 {
		return nil, errors.New("kafkax: no brokers")
	}
	cl, err := kgo.NewClient(cfg.opts()...)
	if err != nil {
		return nil, err
	}
	return &Producer{client: cl}, nil
}

// Publish implements Publisher. It waits for every record's acknowledgement.
func (p *Producer) Publish(ctx context.Context, msgs []Message) []error {
	errs := make([]error, len(msgs))
	var wg sync.WaitGroup
	wg.Add(len(msgs))
	for i, m := range msgs {
		r := &kgo.Record{Topic: m.Topic, Key: []byte(m.Key), Value: m.Value}
		for k, v := range m.Headers {
			r.Headers = append(r.Headers, kgo.RecordHeader{Key: k, Value: []byte(v)})
		}
		p.client.Produce(ctx, r, func(_ *kgo.Record, err error) {
			errs[i] = err
			wg.Done()
		})
	}
	wg.Wait()
	return errs
}

// Ping checks broker connectivity.
func (p *Producer) Ping(ctx context.Context) error { return p.client.Ping(ctx) }

// Close flushes and closes the client.
func (p *Producer) Close() {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = p.client.Flush(ctx)
	p.client.Close()
}

// NewGroupClient creates a consumer-group client with auto-commit disabled
// (callers commit after processing: at-least-once).
func NewGroupClient(cfg Config, group string, topics ...string) (*kgo.Client, error) {
	if len(cfg.Brokers) == 0 || group == "" || len(topics) == 0 {
		return nil, errors.New("kafkax: brokers, group and topics are required")
	}
	o := append(cfg.opts(),
		kgo.ConsumerGroup(group),
		kgo.ConsumeTopics(topics...),
		kgo.DisableAutoCommit(),
		kgo.BlockRebalanceOnPoll(),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()),
		kgo.FetchMaxBytes(16<<20),
	)
	return kgo.NewClient(o...)
}
