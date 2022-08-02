# k8s-client-go

Minimal Go Kubernetes client based on Generics

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
    endpointsOperator := client.NewEndpointsOperator(kc)
    endpoints, err := endpointsOperator.Get(ctx, "kube-system", "kubelet", client.GetOptions{})
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("%+v", endpoints)
}
```

See more in [Examples](https://github.com/castai/k8s-client-go/blob/master/client_test.go#L10)

## Use cases

* Embedding in Go applications for minimal binary size overhead.
* Service discovery by listing and watching [endpoints](https://kubernetes.io/docs/reference/kubernetes-api/service-resources/endpoints-v1/). See [kuberesolver](https://github.com/sercand/kuberesolver) as example for gRPC client side load balancing.