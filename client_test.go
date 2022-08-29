package client

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "github.com/castai/k8s-client-go/types/core/v1"
	metav1 "github.com/castai/k8s-client-go/types/meta/v1"
)

func TestClientAPIGet(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("req", r.URL.String())
		expectedURL := "/api/v1/namespaces/test/endpoints/endpoint1"
		if r.URL.String() != expectedURL {
			t.Fatalf("expected request url %q, got %q", expectedURL, r.URL.String())
		}

		endpoints := corev1.Endpoints{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "endpoint1",
				Namespace: "test",
			},
		}
		if err := json.NewEncoder(w).Encode(endpoints); err != nil {
			t.Fatal(err)
		}
	}))
	defer srv.Close()
	client := &mockClient{
		apiServerURL: srv.URL,
		hc:           &http.Client{Timeout: 5 * time.Second},
	}

	api := NewObjectAPI[corev1.Endpoints](client)

	ns := "test"
	endpointsName := "endpoint1"
	res, err := api.Get(context.Background(), ns, endpointsName, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Namespace != ns {
		t.Fatalf("expected ns %q, got %q", ns, res.Namespace)
	}
	if res.Name != endpointsName {
		t.Fatalf("expected name %q, got %q", endpointsName, res.Name)
	}
}

type mockClient struct {
	apiServerURL string
	hc           *http.Client
}

func (m *mockClient) Do(req *http.Request) (*http.Response, error) {
	return m.hc.Do(req)
}

func (m *mockClient) Token() string {
	return "token"
}

func (m *mockClient) APIServerURL() string {
	return m.apiServerURL
}
