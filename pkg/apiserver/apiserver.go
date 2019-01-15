/*
Copyright 2016 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package apiserver

import (
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/version"
	openapinamer "k8s.io/apiserver/pkg/endpoints/openapi"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	openapicommon "k8s.io/kube-openapi/pkg/common"
)

type SchemeInstallFunc func(*runtime.Scheme)

func NewScheme(installers runtime.SchemeBuilder) *runtime.Scheme {
	scheme := runtime.NewScheme()

	for _, installer := range installers {
		installer(scheme)
	}

	// we need to add the options to empty v1
	// TODO fix the server code to avoid this
	metav1.AddToGroupVersion(scheme, schema.GroupVersion{Version: "v1"})

	// TODO: keep the generic API server from wanting this
	unversioned := schema.GroupVersion{Group: "", Version: "v1"}
	scheme.AddUnversionedTypes(unversioned,
		&metav1.Status{},
		&metav1.APIVersions{},
		&metav1.APIGroupList{},
		&metav1.APIGroup{},
		&metav1.APIResourceList{},
	)

	return scheme
}

// StorageInfo describes storage for a particular API resource.
type StorageInfo struct {
	// Storage is the actual underlying storage implementation.
	// It may (and should) implement some other interfaces,
	// like rest.Getter.
	Storage rest.Storage
	// Resource is the GroupVersionResource represented by this piece of storage.
	Resource schema.GroupVersionResource
}

type Config struct {
	genericConfig *genericapiserver.Config
	// Scheme is the scheme containing the API objects
	// that we care about serializing/deserializing.
	Scheme *runtime.Scheme
	// ParameterScheme is the scheme containing parameter objects.
	// if unset, it will default to the scheme used to construct metav1.ParameterCodec,
	// otherwise, it should (generally) contain the necessary objects from metav1.ParameterCodec.
	ParameterScheme *runtime.Scheme
	// Storage is the storage pieces used to implement the API server.
	// Each bit of storage may implement multiple interface rest.XYZ interfaces
	// to implement different operations.
	Storage []StorageInfo
	// OpenAPIDefinitions fetches generated OpenAPI definitions, if present.
	OpenAPIDefinitions openapicommon.GetOpenAPIDefinitions
}

// GenericConfig instantiates a new generic API server config in the default configuration,
// if not already done.  It's idempotent.
func (cfg *Config) GenericConfig() *genericapiserver.Config {
	if cfg.genericConfig == nil {
		cfg.genericConfig = genericapiserver.NewConfig( /* cheat a bit -- this just fills in Serializer, which we do later in complete */ serializer.CodecFactory{})
	}
	return cfg.genericConfig
}

// Proxyserver contains required state for the Kubernetes bits of the API server.
type BaseAPIServer struct {
	GenericAPIServer *genericapiserver.GenericAPIServer
}

type completedConfig struct {
	GenericConfig genericapiserver.CompletedConfig
	BaseConfig    Config
	// Codecs is the codec factory constructed from the passed-in scheme
	Codecs serializer.CodecFactory
	// ParameterCodec is the parameter codec used to decode parameters passed to the API.
	// By default, we use the standard parameter codec (metav1.ParameterCodec).
	// TODO(directxman12): support using non-standard parameter codecs.
	ParameterCodec runtime.ParameterCodec
}

// CompletedConfig contains a fully populated configuration for the Kubernetes bits
// of the API server.
type CompletedConfig struct {
	// Embed a private pointer that cannot be instantiated outside of this package.
	*completedConfig
}

// Complete fills in any fields not set that are required to have valid data.
// It mutates the receiver.
func (cfg *Config) Complete() CompletedConfig {
	codecs := serializer.NewCodecFactory(cfg.Scheme)
	cfg.genericConfig.Serializer = codecs
	if cfg.OpenAPIDefinitions != nil {
		cfg.genericConfig.OpenAPIConfig = genericapiserver.DefaultOpenAPIConfig(cfg.OpenAPIDefinitions, openapinamer.NewDefinitionNamer(cfg.Scheme))
		cfg.genericConfig.OpenAPIConfig.Info.Title = "Kubernetes metrics-server"
		cfg.genericConfig.OpenAPIConfig.Info.Version = strings.Split(cfg.genericConfig.Version.String(), "-")[0] // TODO(directxman12): remove this once autosetting this doesn't require security definitions
		cfg.genericConfig.SwaggerConfig = genericapiserver.DefaultSwaggerConfig()
	}

	// TODO(directxman12): allow configuring
	cfg.genericConfig.Version = &version.Info{
		Major: "1",
		Minor: "0",
	}

	c := completedConfig{
		GenericConfig: cfg.genericConfig.Complete(nil /* TODO(directxman12): plumb through informers when we need theme */),
		BaseConfig:    *cfg,
	}

	c.Codecs = codecs
	c.ParameterCodec = metav1.ParameterCodec
	if c.BaseConfig.ParameterScheme != nil {
		c.ParameterCodec = runtime.NewParameterCodec(c.BaseConfig.ParameterScheme)
	}

	return CompletedConfig{&c}
}

// New returns a new instance of BaseAPIServer from the given config.
func (c completedConfig) New(name string) (*BaseAPIServer, error) {
	genericServer, err := c.GenericConfig.New(name, genericapiserver.NewEmptyDelegate())
	if err != nil {
		return nil, err
	}

	s := &BaseAPIServer{
		GenericAPIServer: genericServer,
	}

	// gather all the storages into the appropriate format for serving in the API
	groupInfos := make(map[string]*genericapiserver.APIGroupInfo)
	for _, storage := range c.BaseConfig.Storage {
		gvr := storage.Resource

		// populate the group info, if not present (it's a pointer)
		groupInfo, apiGroupPresent := groupInfos[gvr.Group]
		if !apiGroupPresent {
			groupInfoBase := genericapiserver.NewDefaultAPIGroupInfo(gvr.Group, c.BaseConfig.Scheme, c.ParameterCodec, c.Codecs)
			groupInfo = &groupInfoBase
			groupInfos[gvr.Group] = groupInfo
		}

		// populate the version map, if not present
		if _, verPresent := groupInfo.VersionedResourcesStorageMap[gvr.Version]; !verPresent {
			groupInfo.VersionedResourcesStorageMap[gvr.Version] = make(map[string]rest.Storage)
		}

		// insert our storage into the resource map
		groupInfo.VersionedResourcesStorageMap[gvr.Version][gvr.Resource] = storage.Storage
	}

	// install the populated group information into the API server
	for _, groupInfo := range groupInfos {
		if err := s.GenericAPIServer.InstallAPIGroup(groupInfo); err != nil {
			return nil, err
		}
	}

	return s, nil
}
