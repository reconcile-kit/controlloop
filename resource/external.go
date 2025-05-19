package resource

const MessageTypeUpdate = "update"
const MessageTypeDelete = "delete"

type ExternalStorage[T Object[T]] interface {
	Create(item T) (T, error)
	Get(shardID string, groupKind GroupKind, objectKey ObjectKey) (T, bool, error)
	List(listOpts ListOpts) ([]T, error)
	ListPending(shardID string, groupKind GroupKind) ([]T, error)
	Update(item T) (T, error)
	UpdateStatus(item T) (T, error)
	Delete(shardID string, groupKind GroupKind, objectKey ObjectKey) error
}

type ExternalListener interface {
	Listen(shardID string, f func(kind GroupKind, objectKey ObjectKey, messageType string, ack func()))
	ClearQueue() error
}
