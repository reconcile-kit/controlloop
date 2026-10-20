package unstructured

import "sync"

type MetadataProvider struct {
	Kind    string
	ShardID string
	Group   string
}

var (
	instanceMetadataProviderMu sync.RWMutex
	instanceMetadataProvider   = MetadataProvider{}
)

func GetGlobalMetadataProvider() MetadataProvider {
	instanceMetadataProviderMu.RLock()
	defer instanceMetadataProviderMu.RUnlock()

	return instanceMetadataProvider
}

// SetGlobalMetadataProvider is used for creation of Unstructured RemoteClients, because on initializing we need to
// set default GK to resource
// !NOTE! need to be used only with mutex to avoid raceconditions
/*
	example of contsructor:

var (
	clientMu      = &sync.Mutex{}
)

func NewOwnUnstructuredClient(
	externalStorage resource.ExternalStorage[*UnstructuredResource],
	md MetadataProvider,
) (UnstructuredClient, error) {
	clientMu.Lock()
	defer clientMu.Unlock()

	SetGlobalMetadataProvider(md)

	client, err := controlloop.NewRemoteClient[*UnstructuredResource](
		externalStorage,
	)
	if err != nil {
		return nil, err
	}

	unstClient := NewUnstructuredClient(client, md)

	return unstClient, nil
}

*/
func SetGlobalMetadataProvider(metadata MetadataProvider) {
	instanceMetadataProviderMu.Lock()
	defer instanceMetadataProviderMu.Unlock()

	instanceMetadataProvider = metadata
}
