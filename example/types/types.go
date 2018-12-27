//go:generate go run ../../vendor/k8s.io/code-generator/cmd/deepcopy-gen/main.go --logtostderr -i . -h ../hack/boilerplate.txt

package types

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/storage/names"
	"context"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// DemoSpec defines the desired state of Demo
type DemoSpec struct {
}

// DemoStatus defines the observed state of Demo.
// It should always be reconstructable from the state of the cluster and/or outside world.
type DemoStatus struct {
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Demo is the Schema for the demos API
// +k8s:openapi-gen=true
type Demo struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   DemoSpec   `json:"spec,omitempty"`
	Status DemoStatus `json:"status,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// DemoList contains a list of Demo
type DemoList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items []Demo    `json:"items"`
}

var _ rest.RESTCreateStrategy = DemoStrategy{}
var _ rest.RESTUpdateStrategy = DemoStrategy{}
var _ rest.RESTDeleteStrategy = DemoStrategy{}

type DemoStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

func (DemoStrategy) NamespaceScoped() bool {
	return true
}

// Create Strategies
func (DemoStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {}

func (DemoStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList { return nil }

func (DemoStrategy) Canonicalize(obj runtime.Object) {}

// Update Strategies
func (DemoStrategy) AllowCreateOnUpdate() bool { return true }

func (DemoStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {}

func (DemoStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList { return nil }

func (DemoStrategy) AllowUnconditionalUpdate() bool { return true }
