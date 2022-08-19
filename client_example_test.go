package client_test

import (
	"context"
	"fmt"

	client "github.com/castai/k8s-client-go"
	corev1 "github.com/castai/k8s-client-go/types/core/v1"
	metav1 "github.com/castai/k8s-client-go/types/meta/v1"
)

func ExampleObjectAPI() {
	kc, err := client.NewInCluster()
	if err != nil {
		// Handle err
		return
	}

	ctx := context.Background()
	endpointsAPI := client.NewObjectAPI[corev1.Endpoints](kc)

	endpoints, err := endpointsAPI.Get(ctx, "kube-system", "kubelet", metav1.GetOptions{})
	if err != nil {
		// Handle err
		return
	}
	fmt.Printf("%+v\n", endpoints)

	events, err := endpointsAPI.Watch(ctx, "kube-system", "kubelet", metav1.ListOptions{})
	if err != nil {
		// Handle err
		return
	}
	for e := range events.ResultChan() {
		fmt.Printf("%s: %+v\n", e.Type, e.Object)
	}

	// Custom types
	podAPI := client.NewObjectAPI[CustomPod](kc)
	pod, err := podAPI.Get(ctx, "kube-system", "core-dns-123", metav1.GetOptions{})
	if err != nil {
		// Handle err
		return
	}
	fmt.Printf("%+v\n", pod)
}

type CustomPod struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
}

func (p CustomPod) GetObjectMeta() metav1.ObjectMeta {
	return p.ObjectMeta
}

func (p CustomPod) GetTypeMeta() metav1.TypeMeta {
	return p.TypeMeta
}

func (p CustomPod) GVR() metav1.GroupVersionResource {
	return metav1.GroupVersionResource{
		Group:    "",
		Version:  "v1",
		Resource: "pods",
	}
}
