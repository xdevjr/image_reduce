// Package queue implementa uma fila de trabalhos com concorrência configurável.
package queue

import "sync"

// Queue processa caminhos de arquivo com um número dinâmico de workers.
type Queue struct {
	mu      sync.Mutex
	jobs    chan string
	max     int
	workers int
	stopCh  chan struct{}
	wg      sync.WaitGroup
	handler func(string)
	closed  bool
}

// New cria uma fila com o handler dado e o número inicial de workers.
func New(handler func(string), max int) *Queue {
	q := &Queue{
		jobs:    make(chan string, 256),
		stopCh:  make(chan struct{}, 256),
		handler: handler,
	}
	q.SetMax(max)
	return q
}

// SetMax ajusta o número de conversões simultâneas em runtime.
func (q *Queue) SetMax(n int) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if n < 1 {
		n = 1
	}
	q.max = n
	for q.workers < n {
		q.workers++
		q.wg.Add(1)
		go q.worker()
	}
	for q.workers > n {
		q.stopCh <- struct{}{}
	}
}

func (q *Queue) worker() {
	defer q.wg.Done()
	for {
		select {
		case <-q.stopCh:
			q.mu.Lock()
			q.workers--
			q.mu.Unlock()
			return
		case path, ok := <-q.jobs:
			if !ok {
				return
			}
			// Verifica se há sinal de parada pendente antes de processar.
			select {
			case <-q.stopCh:
				q.mu.Lock()
				q.workers--
				q.mu.Unlock()
				return
			default:
			}
			q.handler(path)
		}
	}
}

// Enqueue adiciona um caminho à fila de processamento.
func (q *Queue) Enqueue(path string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return
	}
	q.jobs <- path
}

// Close encerra a fila e aguarda os workers terminarem.
func (q *Queue) Close() {
	q.mu.Lock()
	if q.closed {
		q.mu.Unlock()
		return
	}
	q.closed = true
	close(q.jobs)
	q.mu.Unlock()
	q.wg.Wait()
}