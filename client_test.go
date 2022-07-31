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
	}

	ctx := context.Background()
	endpoints, err := client.GetEndpoints(kc, ctx, "kube-system", "kubelet")
	if err != nil {
		// Handle err
	}

	fmt.Printf("%+v", endpoints)
}
