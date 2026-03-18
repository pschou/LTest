package topk

import (
	"context"
	"math"
	"sync"
	"time"
)

type TopK struct {
	size, keep int
	timeout    time.Duration

	workers *worker
	kept    *worker
	l       sync.Mutex

	ctx     context.Context
	cancel  context.CancelFunc
	reaper  sync.Once
	current chan struct{}
	wg      sync.WaitGroup
}

type worker struct {
	cancel context.CancelFunc
	done   func()
	st     time.Time
	d      time.Duration
	next   *worker
}

type DoneFunc func(Success bool)

// New creates a TopK group
// The limit parameter is the maximum amount of
// goroutines which can be started concurrently.
func New(ctx context.Context, limit, keep int, timeout time.Duration) *TopK {
	size := math.MaxInt32 // 2^32 - 1
	if limit > 0 {
		size = limit
	}

	cctx, cancel := context.WithCancel(ctx)

	k := TopK{
		size:    size,
		keep:    keep,
		timeout: timeout,

		workers: &worker{},
		kept:    &worker{},

		ctx:     cctx,
		cancel:  cancel,
		current: make(chan struct{}, size),
		wg:      sync.WaitGroup{},
	}

	return &k
}

// Add increments the internal WaitGroup counter.
// It can be blocking if the limit of spawned goroutines
// has been reached. It will stop blocking when Done is
// been called.
//
// See sync.WaitGroup documentation for more information.
func (s *TopK) Add() (context.Context, DoneFunc) {
	select {
	case <-s.ctx.Done():
		return s.ctx, func(bool) {} // Context has been cancelled
	case s.current <- struct{}{}:
		break // When there is a slot available, proceed
	}

	cctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	s.wg.Add(1)
	w := &worker{cancel: cancel, st: time.Now(), done: sync.OnceFunc(func() { s.wg.Add(-1) })}

	// Go to the end and add as last element
	current := s.workers
	for current.next != nil {
		current = current.next
	}
	current.next, w.next = w, current.next

	return cctx, func(success bool) {
		w.d = time.Since(w.st)
		//w.cancel() // Ensure the job is done.

		// Remove thyself from workers
		current := s.workers
		for current.next != nil {
			if current.next == w {
				current.next, w.next = w.next, current.next
				break
			}
			current = current.next
		}

		if success {
			// Insert into list kept or drop
			cur, i := s.kept, 0
			for ; i < s.keep; cur, i = cur.next, i+1 {
				if cur.next == nil {
					cur.next, w.next = w, cur.next
					break
				} else if cur.next.d > w.d {
					cur.next, w.next = w, cur.next
					break
				}
			}
			for ; cur.next != nil && i < s.keep; cur, i = cur.next, i+1 {
				// Walk to the end
			}
			if i == s.keep {
				s.timeout = cur.d // Set the new timeout to our oldest
				if cur.next != nil {
					cur.next = nil // Truncate
				}
				s.reaper.Do(func() {
					// Infinite for loop which reaps long lived processes until the context is cancelled.
					go func() {
						for {
							select {
							case <-s.ctx.Done(): // Once cancelled, no more need to clean up.
								for cur := s.workers.next; cur != nil; cur = cur.next {
									if time.Since(cur.st) > s.timeout {
										cur.done() // Unblock everything
									}
								}
								return
							default:
								for cur := s.workers.next; cur != nil; cur = cur.next {
									if time.Since(cur.st) > s.timeout {
										cur.cancel() // Cancel anything too slow
									}
								}
								time.Sleep(time.Second / 10)
							}
						}
					}()
				})
			}
		}

		w.done()
		<-s.current // Release the next job.
	}
}

// Wait blocks until the SizedWaitGroup counter is zero.
// See sync.WaitGroup documentation for more information.
func (s *TopK) Wait() {
	s.wg.Wait()
	s.cancel()
}
