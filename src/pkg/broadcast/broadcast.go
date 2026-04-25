package broadcast

import (
	"sync"
)

type Broadcast[T any] struct {
	listeners []chan T
	mutex     sync.Mutex
}

func NewBroadcast[T any]() Broadcast[T] {
	return Broadcast[T]{
		listeners: make([]chan T, 0),
		mutex:     sync.Mutex{},
	}
}

func (b *Broadcast[T]) AddListener() chan T {
	ch := make(chan T, 1)
	b.mutex.Lock()
	b.listeners = append(b.listeners, ch)
	b.mutex.Unlock()
	return ch
}

func (b *Broadcast[T]) RemoveListener(channel chan T) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	for i, listener := range b.listeners {
		if listener == channel {
			b.listeners = append(b.listeners[:i], b.listeners[i+1:]...)
			close(channel)
			break
		}
	}
}

func (b *Broadcast[T]) Send(update T) {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	for _, channel := range b.listeners {
		select {
		case channel <- update:
		default:
		}
	}
}
