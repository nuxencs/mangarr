package semaphore

type Semaphore interface {
	Acquire()
	Release()
}

type semaphore struct {
	ch chan struct{}
}

// NewWeighted creates a new weighted semaphore with the given
// maximum combined weight for concurrent access.
func NewWeighted(weight int) Semaphore {
	return &semaphore{
		ch: make(chan struct{}, weight),
	}
}

// Acquire acquires the semaphore with a weight of 1, blocking until resources are available.
func (s *semaphore) Acquire() {
	s.ch <- struct{}{}
}

// Release releases the semaphore with a weight of 1.
func (s *semaphore) Release() {
	<-s.ch
}
