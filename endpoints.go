package client

import (
	"context"
	"fmt"
)

var _ ObjectOperator[*Endpoints] = (*EndpointsOperator)(nil)

func NewEndpointsOperator(kc *Client) *EndpointsOperator {
	return &EndpointsOperator{
		kc: kc,
	}
}

type EndpointsOperator struct {
	kc *Client
}

func (e *EndpointsOperator) Get(ctx context.Context, namespace, name string, opts GetOptions) (*Endpoints, error) {
	reqURL := fmt.Sprintf("%s/api/v1/namespaces/%s/endpoints/%s", e.kc.Host, namespace, name)
	return get[*Endpoints](e.kc, ctx, reqURL, opts)
}

func (e *EndpointsOperator) Watch(ctx context.Context, namespace, name string, opts ListOptions) (WatchInterface[*Endpoints], error) {
	reqURL := fmt.Sprintf("%s/api/v1/watch/namespaces/%s/endpoints/%s", e.kc.Host, namespace, name)
	return watch[*Endpoints](e.kc, ctx, reqURL, opts)
}
