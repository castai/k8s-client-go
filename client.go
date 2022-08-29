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
	"os"
	"path"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"

	corev1 "github.com/castai/k8s-client-go/types/core/v1"
	metav1 "github.com/castai/k8s-client-go/types/meta/v1"
)

const (
	serviceAccountToken  = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	serviceAccountCACert = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
)

// Interface is minimal kubernetes Client interface.
type Interface interface {
	// Do sends HTTP request to ObjectAPI server.
	Do(req *http.Request) (*http.Response, error)
	// Token returns current access token.
	Token() string
	// APIServerURL returns API server URL.
	APIServerURL() string
}

// NewInCluster creates Client if it is inside Kubernetes.
func NewInCluster() (*DefaultClient, error) {
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

	client := &DefaultClient{
		apiServerURL: "https://" + net.JoinHostPort(host, port),
		token:        string(token),
		HttpClient:   httpClient,
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

type DefaultClient struct {
	HttpClient *http.Client
	//Logger     Logger
	apiServerURL string

	tokenMu sync.RWMutex
	token   string
}

func (kc *DefaultClient) Do(req *http.Request) (*http.Response, error) {
	return kc.HttpClient.Do(req)
}

func (kc *DefaultClient) Token() string {
	kc.tokenMu.RLock()
	defer kc.tokenMu.RUnlock()

	return kc.token
}

func (kc *DefaultClient) APIServerURL() string {
	return kc.apiServerURL
}

type ResponseDecoderFunc func(r io.Reader) ResponseDecoder

type ObjectAPIOption func(opts *objectAPIOptions)
type objectAPIOptions struct {
	log                Logger
	responseDecodeFunc ResponseDecoderFunc
}

func WithLogger(log Logger) ObjectAPIOption {
	return func(opts *objectAPIOptions) {
		opts.log = log
	}
}

func WithResponseDecoder(decoderFunc ResponseDecoderFunc) ObjectAPIOption {
	return func(opts *objectAPIOptions) {
		opts.responseDecodeFunc = decoderFunc
	}
}

func NewObjectAPI[T corev1.Object](kc Interface, opt ...ObjectAPIOption) ObjectAPI[T] {
	opts := objectAPIOptions{
		log: &DefaultLogger{},
		responseDecodeFunc: func(r io.Reader) ResponseDecoder {
			return json.NewDecoder(r)
		},
	}
	for _, o := range opt {
		o(&opts)
	}

	return &objectAPI[T]{
		kc:   kc,
		opts: opts,
	}
}

type objectAPI[T corev1.Object] struct {
	kc   Interface
	opts objectAPIOptions
}

func buildRequestURL(apiServerURL string, gvr metav1.GroupVersionResource, namespace, name string) string {
	var gvrPath string
	if gvr.Group == "" {
		gvrPath = path.Join("api", gvr.Version)
	} else {
		gvrPath = path.Join("apis", gvr.Group, gvr.Version)
	}
	var nsPath string
	if namespace != "" {
		nsPath = path.Join("namespaces", namespace)
	}
	return apiServerURL + "/" + path.Join(gvrPath, nsPath, gvr.Resource, name)
}

func (o *objectAPI[T]) Get(ctx context.Context, namespace, name string, opts metav1.GetOptions) (*T, error) {
	var t T
	reqURL := buildRequestURL(o.kc.APIServerURL(), t.GVR(), namespace, name)
	req, err := o.getRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	resp, err := o.kc.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid response code %d for request url %q: %s", resp.StatusCode, reqURL, errmsg)
	}
	if err := o.opts.responseDecodeFunc(resp.Body).Decode(&t); err != nil {
		return nil, err
	}
	return &t, err
}

func (o *objectAPI[T]) Watch(ctx context.Context, namespace, name string, opts metav1.ListOptions) (WatchInterface[T], error) {
	var t T
	reqURL := buildRequestURL(o.kc.APIServerURL(), t.GVR(), namespace, name)
	req, err := o.getRequest(ctx, reqURL)
	if err != nil {
		return nil, err
	}
	resp, err := o.kc.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		defer resp.Body.Close()
		errmsg, _ := ioutil.ReadAll(resp.Body)
		return nil, fmt.Errorf("invalid response code %d for request url %q: %s", resp.StatusCode, reqURL, errmsg)
	}
	return newStreamWatcher[T](resp.Body, o.opts.log, o.opts.responseDecodeFunc(resp.Body)), nil
}

func (o *objectAPI[T]) getRequest(ctx context.Context, url string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	if token := o.kc.Token(); len(token) > 0 {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req, nil
}
