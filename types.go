package client

import (
	"context"

	corev1 "github.com/castai/k8s-client-go/types/core/v1"
	metav1 "github.com/castai/k8s-client-go/types/meta/v1"
)

// ObjectGetter is generic object getter.
type ObjectGetter[T corev1.Object] interface {
	Get(ctx context.Context, namespace, name string, _ metav1.GetOptions) (*T, error)
}

// ObjectWatcher is generic object watcher.
type ObjectWatcher[T corev1.Object] interface {
	Watch(ctx context.Context, namespace, name string, _ metav1.ListOptions) (WatchInterface[T], error)
}

// ObjectAPI wraps all operations on object.
type ObjectAPI[T corev1.Object] interface {
	ObjectGetter[T]
	ObjectWatcher[T]
}
