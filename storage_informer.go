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
}

func NewStorageInformer(shardID string, listener resource.ExternalListener, receivers []Receiver) *StorageInformer {
	res := map[resource.GroupKind]Receiver{}
	for _, receiver := range receivers {
		res[receiver.GetGroupKind()] = receiver
	}

	return &StorageInformer{
		res:      res,
		listener: listener,
		shardID:  shardID,
	}
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

func (s *StorageInformer) receiveMessages(ctx context.Context, kind resource.GroupKind, objectKey resource.ObjectKey, messageType string, ack func()) {

	go func() {
		err := s.currentReceive(ctx, kind, objectKey, messageType)
		if err == nil {
			ack()
		}
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
