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

	// Getter example.
	endpointsOperator := client.NewEndpointsOperator(kc)
	endpoints, err := endpointsOperator.Get(ctx, "kube-system", "kubelet", client.GetOptions{})
	if err != nil {
		// Handle err
		return
	}
	fmt.Printf("%+v\n", endpoints)

	// Watcher example.
	events, err := endpointsOperator.Watch(ctx, "kube-system", "kubelet", client.ListOptions{})
	if err != nil {
		// Handle err
		return
	}
	for e := range events.ResultChan() {
		fmt.Printf("%s: %+v\n", e.Type, e.Object)
	}
}
