package client

import (
	"fmt"
	"io"
	"sync"

	corev1 "github.com/castai/k8s-client-go/types/core/v1"
)

// ResponseDecoder allows to specify custom JSON response decoder. By default, std json decoder is used.
type ResponseDecoder interface {
	Decode(v any) error
}

// WatchInterface can be implemented by anything that knows how to Watch and report changes.
type WatchInterface[T corev1.Object] interface {
	// Stop stops watching. Will close the channel returned by ResultChan(). Releases
	// any resources used by the Watch.
	Stop()

	// ResultChan returns a chan which will receive all the events. If an error occurs
	// or Stop() is called, this channel will be closed, in which case the
	// Watch should be completely cleaned up.
	ResultChan() <-chan corev1.Event[T]
}

// StreamWatcher turns any stream for which you can write a Decoder interface
// into a Watch.Interface.
type streamWatcher[T corev1.Object] struct {
	result  chan corev1.Event[T]
	r       io.ReadCloser
	log     Logger
	decoder ResponseDecoder
	sync.Mutex
	stopped bool
}

// NewStreamWatcher creates a StreamWatcher from the given io.ReadClosers.
func newStreamWatcher[T corev1.Object](r io.ReadCloser, log Logger, decoder ResponseDecoder) WatchInterface[T] {
	sw := &streamWatcher[T]{
		r:       r,
		log:     log,
		decoder: decoder,
		result:  make(chan corev1.Event[T]),
	}
	go sw.receive()
	return sw
}

// ResultChan implements Interface.
func (sw *streamWatcher[T]) ResultChan() <-chan corev1.Event[T] {
	return sw.result
}

// Stop implements Interface.
func (sw *streamWatcher[T]) Stop() {
	sw.Lock()
	defer sw.Unlock()
	if !sw.stopped {
		sw.stopped = true
		sw.r.Close()
	}
}

// stopping returns true if Stop() was called previously.
func (sw *streamWatcher[T]) stopping() bool {
	sw.Lock()
	defer sw.Unlock()
	return sw.stopped
}

// receive reads result from the decoder in a loop and sends down the result channel.
func (sw *streamWatcher[T]) receive() {
	defer close(sw.result)
	defer sw.Stop()
	for {
		obj, err := sw.Decode()
		if err != nil {
			// Ignore expected error.
			if sw.stopping() {
				return
			}
			switch err {
			case io.EOF:
				// Watch closed normally.
			case io.ErrUnexpectedEOF:
				sw.log.Infof("k8s-client-go: unexpected EOF during Watch stream event decoding: %v", err)
			default:
				sw.log.Infof("k8s-client-go: unable to decode an event from the Watch stream: %v", err)
			}
			return
		}
		sw.result <- obj
	}
}

// Decode blocks until it can return the next object in the writer. Returns an error
// if the writer is closed or an object can't be decoded.
func (sw *streamWatcher[T]) Decode() (corev1.Event[T], error) {
	var t corev1.Event[T]
	if err := sw.decoder.Decode(&t); err != nil {
		return t, err
	}
	switch t.Type {
	case corev1.EventTypeAdded, corev1.EventTypeModified, corev1.EventTypeDeleted, corev1.EventTypeError:
		return t, nil
	default:
		return t, fmt.Errorf("got invalid Watch event type: %v", t.Type)
	}
}
