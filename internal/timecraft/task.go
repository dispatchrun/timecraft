package timecraft

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskScheduler schedules tasks.
//
// A task is a small unit of work. A process (managed by the Executor) is
// responsible for executing one or more tasks. The management of processes to
// execute tasks and the scheduling of tasks across processes are both
// implementation details.
//
// At this time a task is equal to one HTTP request. Additional types of
// work may be added in the future.
type TaskScheduler struct {
	Executor *Executor

	queue chan<- *TaskInfo
	tasks map[TaskID]*TaskInfo

	// TODO: for now tasks are handled by exactly one process. Add a pool of
	//  processes and then load balance tasks across them
	processes map[processKey]ProcessID

	once   sync.Once
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	mu     sync.Mutex
}

// TaskID is a task identifier.
type TaskID = uuid.UUID

// TaskState is the state of a task.
type TaskState int

const (
	// Queued indicates that the task is waiting to be scheduled.
	Queued TaskState = iota + 1

	// Initializing indicates that the task is in the process of being scheduled
	// on to a process.
	Initializing

	// Executing indicates that the task is currently being executed.
	Executing

	// Error indicates that the task failed with an error. This is a terminal
	// status.
	Error

	// Done indicates that the task executed successfully. This is a terminal
	// status.
	Done
)

// TaskInfo is information about a task.
type TaskInfo struct {
	id         TaskID
	createdAt  time.Time
	state      TaskState
	moduleSpec ModuleSpec
	logSpec    *LogSpec
	req        HTTPRequest
	res        HTTPResponse
	err        error
}

type processKey struct {
	path string
}

// SubmitTask submits a task for execution.
//
// The method returns a TaskID that can be used to query task status and
// results.
func (s *TaskScheduler) SubmitTask(moduleSpec ModuleSpec, logSpec *LogSpec, req HTTPRequest) (TaskID, error) {
	s.once.Do(s.init)

	task := &TaskInfo{
		id:         uuid.New(),
		createdAt:  time.Now(),
		state:      Queued,
		moduleSpec: moduleSpec,
		logSpec:    logSpec,
		req:        req,
	}

	s.synchronize(func() {
		s.tasks[task.id] = task
	})

	s.queue <- task

	return task.id, nil
}

func (s *TaskScheduler) init() {
	s.tasks = map[TaskID]*TaskInfo{}
	s.processes = map[processKey]ProcessID{}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	queue := make(chan *TaskInfo)
	s.queue = queue

	// TODO: spawn many goroutines to schedule tasks
	s.wg.Add(1)
	go s.scheduleLoop(queue)
}

func (s *TaskScheduler) scheduleLoop(queue <-chan *TaskInfo) {
	defer s.wg.Done()

	for {
		select {
		case <-s.ctx.Done():
			return
		case task := <-queue:
			s.scheduleTask(task)
		}
	}
}

func (s *TaskScheduler) scheduleTask(task *TaskInfo) {
	// TODO: add other parts of the ModuleSpec to the key
	key := processKey{path: task.moduleSpec.Path}

	var p ProcessInfo
	var processID ProcessID
	var ok bool
	var err error

	s.synchronize(func() {
		task.state = Initializing
		processID, ok = s.processes[key]
	})
	if ok {
		// Check that the process is still alive.
		p, ok = s.Executor.Lookup(processID)
	}

	if !ok {
		// TODO: use singleflight to initialize the process
		processID, err = s.Executor.Start(task.moduleSpec, task.logSpec)
		if err != nil {
			s.synchronize(func() {
				task.state = Error
				task.err = err
			})
			return
		}
		p, ok = s.Executor.Lookup(processID)
		if !ok {
			s.synchronize(func() {
				task.state = Error
				task.err = errors.New("failed to start process")
			})
			return
		}
		s.synchronize(func() {
			s.processes[key] = processID
		})
	}

	// TODO: better handling of race condition between spawning process and it
	//  being ready to take on work
	time.Sleep(3 * time.Second)

	client := http.Client{
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
				var d net.Dialer
				return d.DialContext(ctx, "unix", p.WorkSocket)
			},
		},
	}

	s.synchronize(func() {
		task.state = Executing
	})

	res, err := client.Do(&http.Request{
		Method: task.req.Method,
		URL:    &url.URL{Scheme: "http", Host: "timecraft", Path: task.req.Path},
		Header: task.req.Headers,
		Body:   io.NopCloser(bytes.NewReader(task.req.Body)),
	})
	if err != nil {
		s.synchronize(func() {
			task.state = Error
			task.err = err
		})
		return
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		s.synchronize(func() {
			task.state = Error
			task.err = err
		})
		return
	}

	s.synchronize(func() {
		task.state = Done
		task.res = HTTPResponse{
			StatusCode: res.StatusCode,
			Headers:    res.Header,
			Body:       body,
		}
	})
}

// Lookup looks up a task by ID.
func (s *TaskScheduler) Lookup(id TaskID) (task TaskInfo, ok bool) {
	s.synchronize(func() {
		var t *TaskInfo
		if t, ok = s.tasks[id]; ok {
			task = *t // copy
		}
	})
	return
}

func (s *TaskScheduler) synchronize(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn()
}

func (s *TaskScheduler) Close() error {
	s.once.Do(s.init)

	s.cancel()
	s.wg.Wait()
	return nil
}
