package client_test

import (
	"context"
	"fmt"

	client "github.com/castai/k8s-client-go"
)

func ExampleClient() {
	kc, err := client.NewInCluster()
	if err != nil {
		// Handle err
		return
	}

	ctx := context.Background()

	// Generic get.
	endpoints, err := client.Get[*client.Endpoints](kc, ctx, "/api/v1/namespaces/kube-system/endpoints/kubelet", client.GetOptions{})
	if err != nil {
		// Handle err
		return
	}

	// Typed methods. Simple wrapper for Get.
	endpoints, err = client.GetEndpoints(kc, ctx, "kube-system", "kubelet", client.GetOptions{})
	if err != nil {
		// Handle err
		return
	}
	fmt.Printf("%+v", endpoints)

	// Watch support.
	events, err := client.Watch[*client.Endpoints](kc, ctx, "/api/v1/namespaces/kube-system/endpoints/kubelet", client.ListOptions{})
	if err != nil {
		// Handle err
		return
	}
	for event := range events.ResultChan() {
		fmt.Println(event.Type, event.Object)
	}
}
