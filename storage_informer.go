package controlloop

import (
	"context"
	"fmt"
	"github.com/reconcile-kit/api/resource"
)

type StorageInformer struct {
	res      map[resource.GroupKind]Receiver
	listener resource.ExternalListener
	shardID  string
	l        Logger
}

// InformerOption configures a StorageInformer.
type InformerOption func(*StorageInformer)

// WithInformerLogger sets where the informer reports events it could not take.
func WithInformerLogger(logger Logger) InformerOption {
	return func(s *StorageInformer) {
		if logger != nil {
			s.l = logger
		}
	}
}

func NewStorageInformer(shardID string, listener resource.ExternalListener, receivers []Receiver, options ...InformerOption) *StorageInformer {
	res := map[resource.GroupKind]Receiver{}
	for _, receiver := range receivers {
		res[receiver.GetGroupKind()] = receiver
	}

	informer := &StorageInformer{
		res:      res,
		listener: listener,
		shardID:  shardID,
		l:        &SimpleLogger{},
	}
	for _, option := range options {
		option(informer)
	}
	return informer
}

func (s *StorageInformer) Run(ctx context.Context) error {
	err := s.listener.ClearQueue(ctx)
	if err != nil {
		return fmt.Errorf("error clearing queue: %w", err)
	}
	for _, receiver := range s.res {
		err := receiver.Init(ctx)
		if err != nil {
			return fmt.Errorf("error initializing receiver: %w, %s %s", err, receiver.GetGroupKind().Group, receiver.GetGroupKind().Kind)
		}
	}
	go func() {
		defer func() {
			if err := recover(); err != nil {
				fmt.Println("receiveMessages Recovered from panic ", err)
			}
		}()
		s.listener.Listen(s.receiveMessages)
	}()
	return nil
}

// receiveMessages takes one event off the queue.
//
// An event is acknowledged only once it has been taken: leaving it
// unacknowledged is what keeps it in the queue for the resend loop to hand out
// again, so a failure here must not be mistaken for a delivery.
//
// It is also reported. Without that this is a silent loss: the resource exists
// in the store, the shard never learns of it, and nothing anywhere says so. The
// only remaining trace is a pending entry in the queue, which is not somewhere
// anyone looks when an object fails to arrive — and it says nothing about why.
func (s *StorageInformer) receiveMessages(ctx context.Context, kind resource.GroupKind, objectKey resource.ObjectKey, messageType string, ack func()) {
	go func() {
		err := s.currentReceive(ctx, kind, objectKey, messageType)
		if err != nil {
			s.l.Error(fmt.Sprintf(
				"controlloop: dropped %s event for %s/%s %s/%s, left unacknowledged for redelivery: %s",
				messageType, kind.Group, kind.Kind, objectKey.Namespace, objectKey.Name, err))
			return
		}
		ack()
	}()
}

func (s *StorageInformer) currentReceive(ctx context.Context, kind resource.GroupKind, objectKey resource.ObjectKey, messageType string) error {
	item, ok := s.res[kind]
	if !ok {
		return nil
	}
	switch messageType {
	case resource.MessageTypeUpdate:
		err := item.Receive(ctx, objectKey)
		if err != nil {
			return err
		}
	case resource.MessageTypeDelete:
		item.Remove(ctx, objectKey)
	}
	return nil
}
