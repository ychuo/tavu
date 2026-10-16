// Package persistqueue serializes local persistence jobs and coalesces queued
// updates by stable key. The most recent accepted update for each key is kept.
package persistqueue

import "sync"

// MaxPendingKeys bounds distinct waiting keys. An executing job is separate,
// so at most MaxPendingKeys+1 jobs are retained. Updating a waiting key never
// consumes another slot, even when the queue is full.
const MaxPendingKeys = 64

type Queue struct {
	mu      sync.Mutex
	wake    *sync.Cond
	idle    *sync.Cond
	running bool
	pending map[string]func() error
	order   []string
	closed  bool
	done    chan struct{}
	onError func(error)
}

// New starts one worker. onError runs on that worker, outside the queue lock;
// UI clients should dispatch its notification onto their UI thread. Neither
// a job nor onError may synchronously call WaitIdle or CloseAndWait on its own queue.
func New(onError func(error)) *Queue {
	q := &Queue{pending: make(map[string]func() error), done: make(chan struct{}), onError: onError}
	q.wake = sync.NewCond(&q.mu)
	q.idle = sync.NewCond(&q.mu)
	go q.run()
	return q
}

// Put never waits for a job to execute. A pending job with the same key is
// replaced by this job, keeping its place in the queue. An executing job is
// allowed to finish, and a later update to its key queues a new job.
//
// False means the job was NOT accepted: the queue is closing, job is nil, or
// MaxPendingKeys different keys are already waiting. Callers must surface a
// rejection or retain the unsaved data; the queue never silently drops a job.
// Jobs should capture immutable values, not mutable application state.
func (q *Queue) Put(key string, job func() error) bool {
	if job == nil {
		return false
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.closed {
		return false
	}
	if _, exists := q.pending[key]; exists {
		q.pending[key] = job
		return true
	}
	if len(q.pending) >= MaxPendingKeys {
		return false
	}
	q.pending[key] = job
	q.order = append(q.order, key)
	q.wake.Signal()
	return true
}

// WaitIdle waits until no job is executing and no job is pending. It leaves
// the queue open and is safe to call concurrently with another waiter or close.
// It includes errors reported synchronously by onError, but cannot wait for
// asynchronous UI callbacks that onError may dispatch. Call it off the UI
// thread. Cond.Wait releases the mutex while waiting; no polling is performed.
//
// A new Put can arrive immediately after WaitIdle returns. Clients using this
// as a save-before-load barrier must suspend mutations while switching projects.
func (q *Queue) WaitIdle() {
	q.mu.Lock()
	defer q.mu.Unlock()
	for q.running || len(q.pending) != 0 {
		q.idle.Wait()
	}
}

// CloseAndWait rejects all later Put calls and waits until every accepted key's
// latest pending job has run. Already-running jobs finish first. Concurrent or
// repeated calls are safe. Call it off the UI thread: filesystem writes may
// take time. No task is discarded when closing.
func (q *Queue) CloseAndWait() {
	q.mu.Lock()
	q.closed = true
	q.wake.Broadcast()
	q.mu.Unlock()
	<-q.done
}

func (q *Queue) run() {
	defer close(q.done)
	for {
		q.mu.Lock()
		for len(q.order) == 0 && !q.closed {
			q.wake.Wait()
		}
		if len(q.order) == 0 {
			q.mu.Unlock()
			return
		}
		key := q.order[0]
		q.order[0] = ""
		q.order = q.order[1:]
		job := q.pending[key]
		delete(q.pending, key)
		q.running = true
		q.mu.Unlock()

		if err := job(); err != nil && q.onError != nil {
			q.onError(err)
		}
		q.mu.Lock()
		q.running = false
		if len(q.pending) == 0 {
			q.idle.Broadcast()
		}
		q.mu.Unlock()
	}
}
