package glocal

import (
	"context"
	"sync"

	"github.com/maestre3d/gluon"
)

type driver struct {
	mu              sync.Mutex
	parentBus       *gluon.Bus
	topicPartitions map[string]*partition // Key: partition_index#topic_name
	schedulerBuffer *schedulerBuffer
	handler         gluon.InternalMessageHandler
	cfg             Configuration
}

var (
	_               gluon.Driver = &driver{}
	defaultDriver   *driver
	driverSingleton = sync.Once{}
)

func init() {
	driverSingleton.Do(func() {
		defaultDriver = &driver{
			mu:              sync.Mutex{},
			topicPartitions: map[string]*partition{},
			schedulerBuffer: newSchedulerBuffer(),
		}
		gluon.Register("local", defaultDriver)
	})
}

func (d *driver) Shutdown(_ context.Context) error {
	d.schedulerBuffer.close()
	return nil
}

func (d *driver) SetParentBus(b *gluon.Bus) {
	d.parentBus = b
	if cfg, ok := d.parentBus.Configuration.Driver.(Configuration); ok {
		d.cfg = cfg
	}
}

func (d *driver) SetInternalHandler(h gluon.InternalMessageHandler) {
	d.handler = h
}

func (d *driver) Publish(_ context.Context, message *gluon.TransportMessage) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	topicPartition := d.topicPartitions[message.Topic]
	if topicPartition == nil {
		topicPartition = newPartition()
		d.topicPartitions[message.Topic] = topicPartition
	}
	topicPartition.push(message)
	d.schedulerBuffer.notify(message.Topic)
	return nil
}

func (d *driver) Subscribe(_ context.Context, _ *gluon.Subscriber) error {
	return nil
}

func (d *driver) Start(_ context.Context) error {
	go d.startSubscriberTaskScheduler()
	return nil
}

func (d *driver) startSubscriberTaskScheduler() {
	for topic := range d.schedulerBuffer.notificationStream {
		go func(t string) {
			if p := defaultDriver.topicPartitions[t]; p != nil {
				subs := d.parentBus.ListSubscribersFromTopic(t)
				for _, sub := range subs {
					_ = d.handler(context.Background(), sub, &p.lastMessage)
				}
			}
		}(topic)
	}
}
