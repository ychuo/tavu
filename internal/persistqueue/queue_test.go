package persistqueue

import (
	"errors"
	"fmt"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func wait(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(3 * time.Second):
		t.Fatal("queue operation exceeded deadline")
	}
}

func closeQueue(t *testing.T, q *Queue) {
	t.Helper()
	done := make(chan struct{})
	go func() { q.CloseAndWait(); close(done) }()
	wait(t, done)
}

func blockWorker(t *testing.T, q *Queue) chan struct{} {
	t.Helper()
	started := make(chan struct{})
	release := make(chan struct{})
	if !q.Put("blocker", func() error { close(started); <-release; return nil }) {
		t.Fatal("blocker rejected")
	}
	wait(t, started)
	return release
}

func TestCoalescesLatestAndPreservesKeyOrder(t *testing.T) {
	q := New(nil)
	release := blockWorker(t, q)
	var order []string
	for i := 0; i < 500; i++ {
		version := i
		if !q.Put("project:a", func() error { order = append(order, fmt.Sprintf("a:%d", version)); return nil }) {
			t.Fatal("same key exhausted capacity")
		}
	}
	q.Put("settings", func() error { order = append(order, "settings"); return nil })
	q.Put("project:b", func() error { order = append(order, "b"); return nil })
	close(release)
	closeQueue(t, q)
	if want := []string{"a:499", "settings", "b"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("got %v, want %v", order, want)
	}
}

func TestRunningKeyCanQueueItsNextVersion(t *testing.T) {
	q := New(nil)
	release := blockWorker(t, q)
	var got int
	if !q.Put("blocker", func() error { got = 1; return nil }) {
		t.Fatal("running key cannot queue new version")
	}
	if !q.Put("blocker", func() error { got = 2; return nil }) {
		t.Fatal("next version cannot be replaced")
	}
	close(release)
	closeQueue(t, q)
	if got != 2 {
		t.Fatalf("final version lost: %d", got)
	}
}

func TestCapacityRejectsExplicitlyButAllowsLatestUpdate(t *testing.T) {
	q := New(nil)
	release := blockWorker(t, q)
	var calls atomic.Int64
	for i := 0; i < MaxPendingKeys; i++ {
		if !q.Put(fmt.Sprint(i), func() error { calls.Add(1); return nil }) {
			t.Fatalf("key %d rejected before capacity", i)
		}
	}
	if q.Put("overflow", func() error { t.Error("rejected job ran"); return nil }) {
		t.Fatal("overflow accepted")
	}
	var latest atomic.Bool
	if !q.Put("0", func() error { latest.Store(true); calls.Add(1); return nil }) {
		t.Fatal("latest existing key rejected at capacity")
	}
	close(release)
	closeQueue(t, q)
	if calls.Load() != MaxPendingKeys || !latest.Load() {
		t.Fatalf("lost accepted job: calls=%d latest=%v", calls.Load(), latest.Load())
	}
}

func TestCloseDrainsAndRejectsLaterWrites(t *testing.T) {
	q := New(nil)
	release := blockWorker(t, q)
	var calls atomic.Int64
	for i := 0; i < 10; i++ {
		q.Put(fmt.Sprint(i), func() error { calls.Add(1); return nil })
	}
	// Set the close state through CloseAndWait, then use the same mutex as
	// the implementation to observe its transition without a timing sleep.
	closed := make(chan struct{})
	go func() { q.CloseAndWait(); close(closed) }()
	deadline := time.NewTimer(3 * time.Second)
	defer deadline.Stop()
	for {
		q.mu.Lock()
		closing := q.closed
		q.mu.Unlock()
		if closing {
			break
		}
		select {
		case <-deadline.C:
			t.Fatal("close did not begin")
		case <-time.After(time.Millisecond):
		}
	}
	if q.Put("late", func() error { return nil }) {
		t.Fatal("accepted write after close began")
	}
	select {
	case <-closed:
		t.Fatal("CloseAndWait returned before active job finished")
	default:
	}
	close(release)
	wait(t, closed)
	if calls.Load() != 10 {
		t.Fatalf("accepted work lost on close: %d", calls.Load())
	}
	closeQueue(t, q)
}

func TestErrorsReportedOutsideLockAndWorkerContinues(t *testing.T) {
	want := errors.New("disk error")
	reported := make(chan struct{})
	var q *Queue
	var next atomic.Bool
	q = New(func(err error) {
		if !errors.Is(err, want) {
			t.Errorf("unexpected error: %v", err)
		}
		// Reentrant Put would deadlock if the error callback held q.mu.
		if !q.Put("retry", func() error { next.Store(true); return nil }) {
			t.Error("callback could not enqueue")
		}
		close(reported)
	})
	q.Put("failure", func() error { return want })
	wait(t, reported)
	closeQueue(t, q)
	if !next.Load() {
		t.Fatal("worker stopped after error")
	}
}

func TestJobExecutesOutsideLock(t *testing.T) {
	q := New(nil)
	enqueued := make(chan struct{})
	var next atomic.Bool
	q.Put("outer", func() error {
		if !q.Put("inner", func() error { next.Store(true); return nil }) {
			t.Error("job could not enqueue")
		}
		close(enqueued)
		return nil
	})
	wait(t, enqueued)
	closeQueue(t, q)
	if !next.Load() {
		t.Fatal("nested queued job lost")
	}
}

func TestConcurrentPutAndClose(t *testing.T) {
	q := New(nil)
	var active atomic.Int64
	var maximum atomic.Int64
	var producers sync.WaitGroup
	for producer := 0; producer < 8; producer++ {
		producers.Add(1)
		go func(id int) {
			defer producers.Done()
			for i := 0; i < 1000; i++ {
				q.Put(fmt.Sprintf("project:%d", id), func() error {
					n := active.Add(1)
					for before := maximum.Load(); n > before && !maximum.CompareAndSwap(before, n); before = maximum.Load() {
					}
					active.Add(-1)
					return nil
				})
			}
		}(producer)
	}
	producers.Wait()
	var closers sync.WaitGroup
	for i := 0; i < 10; i++ {
		closers.Add(1)
		go func() { defer closers.Done(); q.CloseAndWait() }()
	}
	done := make(chan struct{})
	go func() { closers.Wait(); close(done) }()
	wait(t, done)
	if maximum.Load() != 1 {
		t.Fatalf("jobs did not execute serially: %d", maximum.Load())
	}
	if q.Put("after", func() error { return nil }) || q.Put("nil", nil) {
		t.Fatal("accepted after close")
	}
}

func TestEmptyQueueCloses(t *testing.T) {
	q := New(nil)
	if q.Put("nil", nil) {
		t.Fatal("nil job accepted")
	}
	closeQueue(t, q)
}

func TestPutRacingWithCloseKeepsEveryFinalAcceptedValue(t *testing.T) {
	q := New(nil)
	var accepted, persisted [8]atomic.Int64
	start := make(chan struct{})
	var producers sync.WaitGroup
	for producer := range accepted {
		producers.Add(1)
		go func(id int) {
			defer producers.Done()
			<-start
			for value := int64(1); value <= 1000; value++ {
				version := value
				if !q.Put(fmt.Sprintf("project:%d", id), func() error { persisted[id].Store(version); return nil }) {
					return
				}
				accepted[id].Store(version)
			}
		}(producer)
	}
	close(start)
	closed := make(chan struct{})
	go func() { q.CloseAndWait(); close(closed) }()
	producers.Wait()
	wait(t, closed)
	for i := range accepted {
		if got, want := persisted[i].Load(), accepted[i].Load(); got != want {
			t.Fatalf("project %d: persisted %d, last accepted %d", i, got, want)
		}
	}
}

func TestWaitIdleDrainsActiveAndPendingWithoutClosing(t *testing.T) {
	q := New(nil)
	release := blockWorker(t, q)
	var saved atomic.Int64
	q.Put("review", func() error { saved.Store(1); return nil })
	q.Put("review", func() error { saved.Store(2); return nil })
	idle := make(chan struct{})
	go func() { q.WaitIdle(); close(idle) }()
	select {
	case <-idle:
		t.Fatal("WaitIdle returned with an active job")
	default:
	}
	close(release)
	wait(t, idle)
	if saved.Load() != 2 {
		t.Fatalf("WaitIdle missed latest pending version: %d", saved.Load())
	}
	if !q.Put("review", func() error { saved.Store(3); return nil }) {
		t.Fatal("WaitIdle closed the queue")
	}
	again := make(chan struct{})
	go func() { q.WaitIdle(); close(again) }()
	wait(t, again)
	if saved.Load() != 3 {
		t.Fatal("subsequent write was not drained")
	}
	closeQueue(t, q)
}

func TestWaitIdleHandlesMultipleWaitersAndConcurrentClose(t *testing.T) {
	q := New(nil)
	release := blockWorker(t, q)
	var group sync.WaitGroup
	for i := 0; i < 16; i++ {
		group.Add(1)
		go func() { defer group.Done(); q.WaitIdle() }()
	}
	group.Add(1)
	go func() { defer group.Done(); q.CloseAndWait() }()
	close(release)
	done := make(chan struct{})
	go func() { group.Wait(); close(done) }()
	wait(t, done)
	// Waiting on a closed, drained queue also returns immediately.
	idle := make(chan struct{})
	go func() { q.WaitIdle(); close(idle) }()
	wait(t, idle)
}

func TestWaitIdleIncludesErrorCallbackEnqueuedTask(t *testing.T) {
	var q *Queue
	var recovered atomic.Bool
	q = New(func(error) {
		q.Put("retry", func() error { recovered.Store(true); return nil })
	})
	q.Put("fail", func() error { return errors.New("failed save") })
	idle := make(chan struct{})
	go func() { q.WaitIdle(); close(idle) }()
	wait(t, idle)
	if !recovered.Load() {
		t.Fatal("WaitIdle returned before callback-enqueued work completed")
	}
	closeQueue(t, q)
}
