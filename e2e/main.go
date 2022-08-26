package main

import (
	"context"
	"fmt"
	"log"

	"golang.org/x/sync/errgroup"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/rest"

	"k8s.io/client-go/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	client "github.com/castai/k8s-client-go"
	clientcorev1 "github.com/castai/k8s-client-go/types/core/v1"
	clientmetav1 "github.com/castai/k8s-client-go/types/meta/v1"
)

const (
	ns = "conformance"
)

func main() {
	inClusterConfig, err := rest.InClusterConfig()
	if err != nil {
		log.Fatalf("getting in cluster config: %v", err)
	}
	nativeClient, err := kubernetes.NewForConfig(inClusterConfig)
	if err != nil {
		log.Fatalf("creating native k8s client: %v", err)
	}

	kc, err := client.NewInCluster()
	if err != nil {
		log.Fatalf("creating client: %v", err)
	}

	if err := testEndpoints(nativeClient, kc); err != nil {
		log.Fatalf("testing endpoints: %v", err)
	}
}

func testEndpoints(nativeClient *kubernetes.Clientset, kc *client.DefaultClient) error {
	testEndpoints, err := createEndpoint(nativeClient)
	if err != nil {
		return fmt.Errorf("creating test endpoint: %w", err)
	}

	endpointsAPI := client.NewObjectAPI[clientcorev1.Endpoints](kc)

	a, err := nativeClient.CoreV1().Endpoints(testEndpoints.Namespace).Get(context.Background(), testEndpoints.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting endpoint using native client: %w", err)
	}

	b, err := endpointsAPI.Get(context.Background(), testEndpoints.Namespace, testEndpoints.Name, clientmetav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("getting endpoint using client: %w", err)
	}

	compare := func(a *corev1.Endpoints, b *clientcorev1.Endpoints) error {
		fmt.Printf("compare endpoints:\na=%+v\nb=%+v\n", a, b)
		ip1 := a.Subsets[0].Addresses[0].IP
		ip2 := b.Subsets[0].Addresses[0].IP
		if ip1 != ip2 {
			return fmt.Errorf("addresse ips are not equal: %v != %v", ip1, ip2)
		}

		port1 := a.Subsets[0].Ports[0]
		port2 := b.Subsets[0].Ports[0]
		if ip1 != ip2 {
			return fmt.Errorf("ports are not equal: %v != %v", port1, port2)
		}
		return nil
	}

	// Compare values from get.
	if err := compare(a, b); err != nil {
		return err
	}

	var errg errgroup.Group
	errg.Go(func() error {
		e, err := nativeClient.CoreV1().Endpoints(testEndpoints.Namespace).Watch(context.Background(), metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("watching using native client: %w", err)
		}
		var deleted, added bool
		for event := range e.ResultChan() {
			if event.Type == watch.Added {
				added = true
			}
			if event.Type == watch.Deleted {
				deleted = true
			}
			if added && deleted {
				if v, ok := event.Object.(*corev1.Endpoints); ok {
					a = v
					e.Stop()
				}
			}
		}
		return nil
	})

	errg.Go(func() error {
		e, err := endpointsAPI.Watch(context.Background(), testEndpoints.Namespace, testEndpoints.Name, clientmetav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("watching using native client: %w", err)
		}
		var deleted, added bool
		for event := range e.ResultChan() {
			if event.Type == clientcorev1.EventTypeAdded {
				added = true
			}
			if event.Type == clientcorev1.EventTypeDeleted {
				deleted = true
			}
			if added && deleted {
				b = event.Object
				e.Stop()
			}
		}
		return nil
	})
	if err := deleteEndpoints(nativeClient, testEndpoints); err != nil {
		return fmt.Errorf("deleting test endpoints: %w", err)
	}
	testEndpoints, err = createEndpoint(nativeClient)
	if err != nil {
		return fmt.Errorf("re-creating test endpoint: %w", err)
	}

	if err := errg.Wait(); err != nil {
		return err
	}

	// Compare values from watch.
	if err := compare(a, b); err != nil {
		return err
	}
	return nil
}

func createEndpoint(c *kubernetes.Clientset) (*corev1.Endpoints, error) {
	return c.CoreV1().Endpoints(ns).Create(context.Background(), &corev1.Endpoints{
		ObjectMeta: metav1.ObjectMeta{
			Name: "test-endpoint",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "10.10.0.15",
					},
				},
				Ports: []corev1.EndpointPort{
					{
						Name: "http",
						Port: 8080,
					},
				},
			},
		},
	}, metav1.CreateOptions{})
}

func deleteEndpoints(c *kubernetes.Clientset, e *corev1.Endpoints) error {
	t := int64(0)
	return c.CoreV1().Endpoints(e.Namespace).Delete(context.Background(), e.Name, metav1.DeleteOptions{GracePeriodSeconds: &t})
}
