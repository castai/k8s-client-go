## About

Minimal Go Kubernetes client based on Generics

## Installing

```
go get github.com/castai/k8s-client-go
```

## Usage

```go
import (
    "context"
    "log"
    "fmt"
    client "github.com/castai/k8s-client-go"
)

func main() {
    kc, err := client.NewInCluster()
    if err != nil {
        log.Fatal(err)
    }
    ctx := context.Backgroud()

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
}
```

See more in [Examples](https://github.com/castai/k8s-client-go/blob/master/client_example_test.go#L10)

## Use cases

* Embedding in Go applications for minimal binary size overhead.
* Service discovery by listing and watching [endpoints](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/endpoints-v1/). See [kuberesolver](https://github.com/sercand/kuberesolver) as example for gRPC client side load balancing.
