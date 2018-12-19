package main

import (
	"context"
	"os"

	"sigs.k8s.io/apiserver-runtime/pkg/builder"
	"sigs.k8s.io/apiserver-runtime/example/types"
	"k8s.io/klog"
	"k8s.io/apimachinery/pkg/runtime"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"

	// pull in auth plugins
	_ "k8s.io/client-go/plugin/pkg/client/auth"
)

var (
	scheme = runtime.NewScheme()
)

func init() {
	scheme.AddKnownTypes(types.GroupVersion, &types.Demo{}, &types.DemoList{})
}

type TestGetter struct {
}

func (t *TestGetter) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	obj := &types.Demo{}
	namespace, _ := genericapirequest.NamespaceFrom(ctx)
	obj.Name = "some-demo-object"
	obj.Namespace = namespace
	
	return obj, nil
}

func (t *TestGetter) New() runtime.Object {
	return &types.Demo{}
}

func (t *TestGetter) NamespaceScoped() bool {
	return true
}

func main() {
	b := &builder.APIServerBase{}
	b.WithScheme(scheme)
	b.WithStorage(types.GroupVersion.WithResource("demos"), &TestGetter{})
	b.Flags().Parse(os.Args)

	defer klog.Flush()
	if err := b.Run(); err != nil {
		klog.Fatalf("unable to run apiserver: %v", err)
	}
}
