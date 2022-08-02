package client

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	serviceAccountToken  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	serviceAccountCACert = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// Interface is minimal kubernetes Client interface.
type Interface interface {
	// Do sends HTTP request to API server.
	Do(req *http.Request) (*http.Response, error)
	// GetRequest prepares HTTP GET request with Authorization header.
	GetRequest(url string) (*http.Request, error)
	// Token returns current access token.
	Token() string
}

// NewInCluster creates Client if it is inside Kubernetes.
func NewInCluster() (*Client, error) {
	host, port := os.Getenv("KUBERNETES_SERVICE_HOST"), os.Getenv("KUBERNETES_SERVICE_PORT")
	if len(host) == 0 || len(port) == 0 {
		return nil, fmt.Errorf("unable to load in-cluster configuration, KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined")
	}
	token, err := ioutil.ReadFile(serviceAccountToken)
	if err != nil {
		return nil, err
	}
	ca, err := ioutil.ReadFile(serviceAccountCACert)
	if err != nil {
		return nil, err
	}
	certPool := x509.NewCertPool()
	certPool.AppendCertsFromPEM(ca)
	transport := &http.Transport{TLSClientConfig: &tls.Config{
		MinVersion: tls.VersionTLS12,
		RootCAs:    certPool,
	}}
	httpClient := &http.Client{Transport: transport, Timeout: time.Nanosecond * 0}

	client := &Client{
		Host:       "https://" + net.JoinHostPort(host, port),
		token:      string(token),
		HttpClient: httpClient,
		ResponseDecoderFunc: func(r io.Reader) ResponseDecoder {
			return json.NewDecoder(r)
		},
		Logger: &DefaultLogger{},
	}

	// Create a new file watcher to listen for new Service Account tokens
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	go func() {
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if event.Op&fsnotify.Write == fsnotify.Write {
					token, err := ioutil.ReadFile(serviceAccountToken)
					if err == nil {
						client.tokenMu.Lock()
						client.token = string(token)
						client.tokenMu.Unlock()
					}
				}
			case _, ok := <-watcher.Errors:
				if !ok {
					return
				}
			}
		}
	}()

	err = watcher.Add(serviceAccountToken)
	if err != nil {
		return nil, err
	}

	return client, nil
}

type Client struct {
	Host                string
	HttpClient          *http.Client
	ResponseDecoderFunc func(r io.Reader) ResponseDecoder
	Logger              Logger

	tokenMu sync.RWMutex
	token   string
}

func (kc *Client) GetRequest(ctx context.Context, url string) (*http.Request, error) {
	kc.ResponseDecoderFunc = func(r io.Reader) ResponseDecoder {
		return json.NewDecoder(r)
	}

	if !strings.HasPrefix(url, kc.Host) {
		url = fmt.Sprintf("%s/%s", kc.Host, url)
	}
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token := kc.Token(); len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}

func (kc *Client) Do(req *http.Request) (*http.Response, error) {
	return kc.HttpClient.Do(req)
}

func (kc *Client) Token() string {
	kc.tokenMu.RLock()
	defer kc.tokenMu.RUnlock()

	return kc.token
}

func Get[T Object](kc *Client, ctx context.Context, reqURL string, _ GetOptions) (T, error) {
	var t T
	u, err := url.Parse(reqURL)
	if err != nil {
		return t, err
	}
	req, err := kc.GetRequest(ctx, u.String())
	if err != nil {
		return t, err
	}
	resp, err := kc.Do(req)
	if err != nil {
		return t, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return t, fmt.Errorf("invalid response code %d for request url %q: %s", resp.StatusCode, reqURL, errmsg)
	}
	if err := kc.ResponseDecoderFunc(resp.Body).Decode(&t); err != nil {
		return t, err
	}
	return t, err
}

func Watch[T Object](kc *Client, ctx context.Context, reqURL string, _ ListOptions) (WatchInterface[T], error) {
	u, err := url.Parse(reqURL)
	if err != nil {
		return nil, err
	}
	req, err := kc.GetRequest(ctx, u.String())
	if err != nil {
		return nil, err
	}
	resp, err := kc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid response code %d for request url %q: %s", resp.StatusCode, reqURL, errmsg)
	}

	return newStreamWatcher[T](resp.Body, kc.Logger, kc.ResponseDecoderFunc(resp.Body)), nil
}

func GetEndpoints(kc *Client, ctx context.Context, namespace, name string, opts GetOptions) (*Endpoints, error) {
	reqURL := fmt.Sprintf("%s/api/v1/namespaces/%s/endpoints/%s", kc.Host, namespace, name)
	return Get[*Endpoints](kc, ctx, reqURL, opts)
}

func WatchEndpoints(kc *Client, ctx context.Context, namespace, name string, _ ListOptions) (WatchInterface[*Endpoints], error) {
	u, err := url.Parse(fmt.Sprintf("%s/api/v1/watch/namespaces/%s/endpoints/%s", kc.Host, namespace, name))
	if err != nil {
		return nil, err
	}
	req, err := kc.GetRequest(ctx, u.String())
	if err != nil {
		return nil, err
	}
	resp, err := kc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid response code %d for service %s in namespace %s: %s", resp.StatusCode, name, namespace, string(errmsg))
	}
	return newStreamWatcher[*Endpoints](resp.Body, kc.Logger, kc.ResponseDecoderFunc(resp.Body)), nil
}
