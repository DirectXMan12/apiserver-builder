/*
Copyright 2017 The Kubernetes Authors.

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
	"fmt"
	"net"

	"sigs.k8s.io/apiserver-runtime/pkg/apiserver"
	genericoptions "k8s.io/apiserver/pkg/server/options"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apiserver/pkg/storage/storagebackend"
)

const (
	DefaultEtcdPathPrefix = "/registry"
)

type BaseAPIServerOptions struct {
	// genericoptions.ReccomendedOptions - EtcdOptions
	SecureServing  *genericoptions.SecureServingOptionsWithLoopback
	Authentication *genericoptions.DelegatingAuthenticationOptions
	Authorization  *genericoptions.DelegatingAuthorizationOptions
	Features       *genericoptions.FeatureOptions
	Etcd           *genericoptions.EtcdOptions
}

func NewBaseAPIServerOptions() *BaseAPIServerOptions {
	secureServingOpts := genericoptions.NewSecureServingOptions()
	// copied from the reccomended options -- allows for lots of multiplexed connections
	// from the aggregator proxy.
	secureServingOpts.HTTP2MaxStreamsPerConnection = 1000

	o := &BaseAPIServerOptions{
		SecureServing:  secureServingOpts.WithLoopback(),
		Authentication: genericoptions.NewDelegatingAuthenticationOptions(),
		Authorization:  genericoptions.NewDelegatingAuthorizationOptions(),
		Features:       genericoptions.NewFeatureOptions(),
		Etcd:           genericoptions.NewEtcdOptions(storagebackend.NewDefaultConfig(DefaultEtcdPathPrefix, nil)),
	}

	o.Authorization.RemoteKubeConfigFileOptional = true
	o.Authentication.RemoteKubeConfigFileOptional = true

	return o
}

func (o BaseAPIServerOptions) Validate(args []string) error {
	var errs []error
	errs = append(errs, o.SecureServing.Validate()...)
	errs = append(errs, o.Authentication.Validate()...)
	errs = append(errs, o.Authorization.Validate()...)
	errs = append(errs, o.Features.Validate()...)
	errs = append(errs, o.Etcd.Validate()...)

	return utilerrors.NewAggregate(errs)
}

func (o *BaseAPIServerOptions) Complete() error {
	return nil
}

func (o BaseAPIServerOptions) ApplyTo(cfg *apiserver.Config) error {
	serverConfig := cfg.GenericConfig()
	// TODO have a "real" external address (have an AdvertiseAddress?)
	if err := o.SecureServing.MaybeDefaultWithSelfSignedCerts("localhost", nil, []net.IP{net.ParseIP("127.0.0.1")}); err != nil {
		return fmt.Errorf("error creating self-signed certificates: %v", err)
	}

	if err := o.SecureServing.ApplyTo(&serverConfig.SecureServing, &serverConfig.LoopbackClientConfig); err != nil {
		return err
	}

	if err := o.Authentication.ApplyTo(&serverConfig.Authentication, serverConfig.SecureServing, serverConfig.OpenAPIConfig); err != nil {
		return err
	}
	if err := o.Authorization.ApplyTo(&serverConfig.Authorization); err != nil {
		return err
	}

	if err := o.Etcd.ApplyTo(serverConfig); err != nil {
		return err
	}

	// TODO: we can't currently serve swagger because we don't have a good way to dynamically update it
	return nil
}
