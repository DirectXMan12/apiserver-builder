package builder

import (
	"k8s.io/apiserver/pkg/registry/generic/registry"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type StorageBuilderFunc func(gr schema.GroupResource, restOptionsGetter generic.RESTOptionsGetter) *registry.Store

type storageBuilder struct {
	store *registry.Store
}

func NewStorageBuilder() *storageBuilder {
	return &storageBuilder{
		store: &registry.Store{},
	}
}

func (b *storageBuilder) WithNewFunc(newFunc func() runtime.Object) *storageBuilder {
	b.store.NewFunc = newFunc
	return b
}

func (b *storageBuilder) WithNewListFunc(newListFunc func() runtime.Object) *storageBuilder {
	b.store.NewListFunc = newListFunc
	return b
}

func (b *storageBuilder) WithCreateStrategy(strategy rest.RESTCreateStrategy) *storageBuilder {
	b.store.CreateStrategy = strategy
	return b
}

func (b *storageBuilder) WithUpdateStrategy(strategy rest.RESTUpdateStrategy) *storageBuilder {
	b.store.UpdateStrategy = strategy
	return b
}

func (b *storageBuilder) WithDeleteStrategy(strategy rest.RESTDeleteStrategy) *storageBuilder {
	b.store.DeleteStrategy = strategy
	return b
}

func (b *storageBuilder) WithAfterCreateFunc(afterCreateFunc registry.ObjectFunc) *storageBuilder {
	b.store.AfterCreate = afterCreateFunc
	return b
}

func (b *storageBuilder) WithAfterUpdateFunc(afterUpdateFunc registry.ObjectFunc) *storageBuilder {
	b.store.AfterUpdate = afterUpdateFunc
	return b
}

func (b *storageBuilder) WithAfterDeleteFunc(afterDeleteFunc registry.ObjectFunc) *storageBuilder {
	b.store.AfterDelete = afterDeleteFunc
	return b
}

func (b *storageBuilder) Build() StorageBuilderFunc {
	return func(gr schema.GroupResource, restOptionsGetter generic.RESTOptionsGetter) *registry.Store {
		b.store.DefaultQualifiedResource = gr
		options := &generic.StoreOptions{RESTOptions: restOptionsGetter}
		if err := b.store.CompleteWithOptions(options); err != nil {
			panic(err) // TODO: Propagate error up
		}
		return b.store
	}
}
