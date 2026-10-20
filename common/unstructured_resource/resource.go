package unstructured

import (
	"encoding/json"

	"github.com/reconcile-kit/api/resource"
)

type UnstructuredResource struct {
	resource.Resource
	MetadataProvider *MetadataProvider `json:"-"`
	Spec             json.RawMessage   `json:"spec"`
	Status           json.RawMessage   `json:"status"`
}

func (a *UnstructuredResource) GetGK() resource.GroupKind {
	md := GetGlobalMetadataProvider()
	if a.MetadataProvider != nil {
		md = *a.MetadataProvider
	}

	return resource.GroupKind{Kind: md.Kind, Group: md.Group}
}

func (a *UnstructuredResource) DeepCopy() *UnstructuredResource {
	return resource.DeepCopyStruct(a).(*UnstructuredResource)
}

func (a *UnstructuredResource) SetMetadataProvider(md *MetadataProvider) {
	a.MetadataProvider = md
	if md == nil {
		return
	}

	a.ResourceGroup = md.Group
	a.Kind = md.Kind
	a.ShardID = md.ShardID
}
