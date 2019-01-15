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

package builder

import (
	"flag"
	"fmt"
	"sync"

	"github.com/spf13/pflag"

	// NB(directxman12): this package runs stuff on init, polluting the global flagset :-/.
	// there's a fix on the way, but for the mean time, we just have to deal with it, because
	// the main k8s.io/apiserver repo imports it anyway.
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apiserver/pkg/registry/rest"
	genericapiserver "k8s.io/apiserver/pkg/server"
	"k8s.io/apiserver/pkg/util/logs"
	"k8s.io/klog"

	"sigs.k8s.io/apiserver-runtime/pkg/apiserver"
)

// APIServerBase provides a base set of functionality for any API server.
// Embed it in a struct containing your options, then:
//
// - Use Flags() to add flags, then call Flags().Parse(os.Argv)
// - Use WithStorage(groupVersionResource, storage) to install storage (or any of
//   the more specific methods).
// - Use Run(stopChannel) to start the server
//
// All methods on this struct are idempotent except for Run -- they'll perform any
// initialization on the first call, then return the existing object on later calls.
// Methods on this struct are not safe to call from multiple goroutines without
// external synchronization.
type APIServerBase struct {
	*BaseAPIServerOptions

	// Name is the name of this API server (used for informational purposes)
	Name string

	// Stop is the stop channel used to stop the API server.  If nil,
	// it will be defaulted to a signal handler.
	Stop <-chan struct{}

	// FlagSet is the flagset to add flags to.
	// It defaults to the normal CommandLine flags
	// if not explicitly set.
	FlagSet *pflag.FlagSet

	// scheme is the scheme to use to serialize/deserialize objects.
	// It's passed to the generated Config.
	scheme *runtime.Scheme

	// flagOnce controls initialization of the flags.
	flagOnce sync.Once

	// TODO(directxman12): add in client, informers helpers as needed by users.
	server *apiserver.BaseAPIServer
	config *apiserver.Config

	// storage is kept here to avoid needed into instantiate config earlier
	storage map[schema.GroupVersionResource]apiserver.StorageInfo
}

// InstallFlags installs the minimum required set of flags into the flagset.
func (b *APIServerBase) InstallFlags() {
	b.initFlagSet()
	b.flagOnce.Do(func() {
		if b.BaseAPIServerOptions == nil {
			b.BaseAPIServerOptions = NewBaseAPIServerOptions()
		}

		b.SecureServing.AddFlags(b.FlagSet)
		b.Authentication.AddFlags(b.FlagSet)
		b.Authorization.AddFlags(b.FlagSet)
		b.Features.AddFlags(b.FlagSet)

		// TODO(directxman12): when we finally fix the "registering flags in init" problem,
		// also call logs.AddFlags, and flag.Set("logtostderr", "true") here.
		var goflagFlagSet flag.FlagSet
		klog.InitFlags(&goflagFlagSet)
		b.FlagSet.AddGoFlagSet(&goflagFlagSet)
	})
}

// initFlagSet populates the flagset to the CommandLine flags if it's not already set.
func (b *APIServerBase) initFlagSet() {
	if b.FlagSet == nil {
		// default to the normal commandline flags
		b.FlagSet = pflag.CommandLine
	}
}

// Flags returns the flagset used by this adapter.
// It will initialize the flagset with the minimum required set
// of flags as well.
func (b *APIServerBase) Flags() *pflag.FlagSet {
	b.initFlagSet()
	b.InstallFlags()

	return b.FlagSet
}

// WithStorage adds the given storage to the set of resources served by this API server, to
// later be passed to the Config when it's instantiated.
func (b *APIServerBase) WithStorage(gvr schema.GroupVersionResource, storage rest.Storage) error {
	if _, exists := b.storage[gvr]; exists {
		return fmt.Errorf("attempting to add duplicate storage for %s", gvr)
	}
	if b.storage == nil {
		b.storage = make(map[schema.GroupVersionResource]apiserver.StorageInfo)
	}

	b.storage[gvr] = apiserver.StorageInfo{
		Resource: gvr,
		Storage:  storage,
	}

	return nil
}

// WithScheme sets the scheme for this API server, adding in extra boilerplate
// objects for API serving (like options).
func (b *APIServerBase) WithScheme(builders runtime.SchemeBuilder) {
	b.scheme = apiserver.NewScheme(builders)
}

// Config fetches the configuration used to ulitmately create the custom metrics adapter's
// API server.  While this method is idempotent, it does "cement" values of some of the other
// fields, so make sure to only call it just before `Server` or `Run`.
// Normal users should not need to call this method -- it's for advanced use cases.
func (b *APIServerBase) Config() (*apiserver.Config, error) {
	if b.config == nil {
		b.InstallFlags() // just to be sure

		config := &apiserver.Config{
			Scheme: b.scheme,
		}
		err := b.BaseAPIServerOptions.ApplyTo(config)
		if err != nil {
			return nil, err
		}
		for _, storageInfo := range b.storage {
			config.Storage = append(config.Storage, storageInfo)
		}
		b.config = config
	}

	return b.config, nil
}

// Server fetches API server object used to ulitmately run the custom metrics adapter.
// While this method is idempotent, it does "cement" values of some of the other
// fields, so make sure to only call it just before `Run`.
// Normal users should not need to call this method -- it's for advanced use cases.
func (b *APIServerBase) Server() (*apiserver.BaseAPIServer, error) {
	if b.server == nil {
		config, err := b.Config()
		if err != nil {
			return nil, err
		}

		if b.Name == "" {
			b.Name = "aggregated-apiserver"
		}

		// we add in the informers if they're not nil, but we don't try and
		// construct them if the user didn't ask for them
		server, err := config.Complete().New(b.Name)
		if err != nil {
			return nil, err
		}
		b.server = server
	}

	return b.server, nil
}

// Run runs this custom metrics adapter until the given stop channel is closed.
// If the stop channel is nil, we use a signal handler instead.
func (b *APIServerBase) Run() error {
	logs.InitLogs()
	server, err := b.Server()
	if err != nil {
		return err
	}

	if b.Stop == nil {
		b.Stop = genericapiserver.SetupSignalHandler()
	}

	return server.GenericAPIServer.PrepareRun().Run(b.Stop)
}
