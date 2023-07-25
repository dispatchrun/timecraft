package timecraft

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/singleflight"
)

// Task scheduler configuration.
// TODO: make this configurable via TaskScheduler and/or via Submit(..)
const (
	// executionTimeout is the time limit for tasks once they're executing.
	executionTimeout = 1 * time.Minute

	// expiryTimeout is the maximum amount of time a task can be
	// queued for before it expires.
	expiryTimeout = 10 * time.Minute

	// threadCount controls the number of threads responsible for executing
	// tasks in the background. This is the maximum concurrency for task
	// execution.
	threadCount = 8
)

// TaskScheduler schedules tasks across processes.
//
// A task is a unit of work. A process (managed by the ProcessManager) is
// responsible for executing one or more tasks. The management of processes to
// execute tasks and the scheduling of tasks across processes are both
// implementation details of the scheduler.
type TaskScheduler struct {
	ProcessManager *ProcessManager

	queue chan<- *TaskInfo
	tasks map[TaskID]*TaskInfo

	// TODO: for now tasks are handled by exactly one process. Add a pool of
	//  processes and then load balance tasks across them
	processes map[string]ProcessID
	pinit     singleflight.Group

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

	// Success indicates that the task executed successfully. This is a terminal
	// status.
	Success
)

// TaskInfo is information about a task.
type TaskInfo struct {
	id          TaskID
	creator     ProcessID
	createdAt   time.Time
	state       TaskState
	processID   ProcessID
	moduleSpec  ModuleSpec
	logSpec     *LogSpec
	input       TaskInput
	output      TaskOutput
	err         error
	ctx         context.Context
	cancel      context.CancelFunc
	completions chan<- TaskID
}

// TaskInput is input for a task.
type TaskInput interface {
	taskInput()
}

// TaskOutput is output from a task.
type TaskOutput interface {
	taskOutput()
}

// Submit submits a task for execution.
//
// The method returns a TaskID that can be passed to Lookup to query the task
// status and fetch task output.
//
// The method accepts an optional channel that receives a completion
// notification once the task is complete (succeeds, or fails permanently).
//
// Once a task is complete, it must be discarded via Discard.
func (s *TaskScheduler) Submit(moduleSpec ModuleSpec, logSpec *LogSpec, input TaskInput, processID ProcessID, completions chan<- TaskID) (TaskID, error) {
	s.once.Do(s.init)

	task := &TaskInfo{
		id:          uuid.New(),
		createdAt:   time.Now(),
		creator:     processID,
		state:       Queued,
		moduleSpec:  moduleSpec,
		logSpec:     logSpec,
		input:       input,
		completions: completions,
	}

	task.ctx, task.cancel = context.WithCancel(s.ctx)

	s.synchronize(func() {
		s.tasks[task.id] = task
	})

	s.queue <- task

	return task.id, nil
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

// Discard discards a task by ID.
func (s *TaskScheduler) Discard(id TaskID) (ok bool) {
	s.synchronize(func() {
		var task *TaskInfo
		if task, ok = s.tasks[id]; ok {
			task.cancel()
			delete(s.tasks, id)
		}
	})
	return
}

func (s *TaskScheduler) init() {
	s.tasks = map[TaskID]*TaskInfo{}
	s.processes = map[string]ProcessID{}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	queue := make(chan *TaskInfo)
	s.queue = queue

	s.wg.Add(threadCount)
	for i := 0; i < threadCount; i++ {
		go s.scheduleLoop(queue)
	}
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
	if time.Since(task.createdAt) > expiryTimeout {
		s.completeTask(task, errors.New("task expired"), nil)
		return
	}

	s.synchronize(func() {
		task.state = Initializing
	})

	key := task.moduleSpec.Key()

	initResult, err, _ := s.pinit.Do(key, func() (any, error) {
		var processID ProcessID
		var process ProcessInfo
		var ok bool
		s.synchronize(func() {
			task.state = Initializing
			processID, ok = s.processes[key]
		})
		if ok {
			// Check that the process is still alive.
			process, ok = s.ProcessManager.Lookup(processID)
		}

		if !ok {
			var err error
			processID, err = s.ProcessManager.Start(task.moduleSpec, task.logSpec)
			if err != nil {
				return nil, err
			}
			process, ok = s.ProcessManager.Lookup(processID)
			if !ok {
				return nil, errors.New("failed to start process")
			}
			s.synchronize(func() {
				s.processes[key] = processID
			})
		}
		return process, nil
	})
	if err != nil {
		s.completeTask(task, err, nil)
		return
	}
	process := initResult.(ProcessInfo)

	s.synchronize(func() {
		task.state = Executing
		task.processID = process.ID
	})

	switch input := task.input.(type) {
	case *HTTPRequest:
		s.executeHTTPTask(&process, task, input)
	default:
		s.completeTask(task, errors.New("invalid task input"), nil)
	}
}

func (s *TaskScheduler) executeHTTPTask(process *ProcessInfo, task *TaskInfo, request *HTTPRequest) {
	client := http.Client{
		Transport: process.Transport,
		Timeout:   executionTimeout,
	}

	request.Headers.Set("User-Agent", "timecraft "+Version())
	request.Headers.Set("X-Timecraft-Task", task.id.String())
	request.Headers.Set("X-Timecraft-Creator", task.creator.String())

	req := (&http.Request{
		Method: request.Method,
		URL: &url.URL{
			Scheme: "http",
			Host:   net.JoinHostPort("127.0.0.1", strconv.Itoa(request.Port)),
			Path:   request.Path,
		},
		Header: request.Headers,
		Body:   io.NopCloser(bytes.NewReader(request.Body)),
		// TODO: disable Transfer-Encoding:chunked temporarily, until we have tracing support
		ContentLength:    int64(len(request.Body)),
		TransferEncoding: []string{"identity"},
	}).WithContext(task.ctx)

	fmt.Println("EXEC", req.Method, req.URL.Path)
	res, err := client.Do(req)
	if err != nil {
		fmt.Println("ERR", err)
		s.completeTask(task, err, nil)
		return
	}

	fmt.Println("RES", res.Status)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		s.completeTask(task, err, nil)
		return
	}

	s.completeTask(task, nil, &HTTPResponse{
		StatusCode: res.StatusCode,
		Headers:    res.Header,
		Body:       body,
	})
}

func (s *TaskScheduler) completeTask(task *TaskInfo, err error, output TaskOutput) {
	s.synchronize(func() {
		if err != nil {
			task.state = Error
			task.err = err
		} else {
			task.state = Success
			task.output = output
		}
	})

	if task.completions != nil {
		select {
		case <-s.ctx.Done():
		case task.completions <- task.id:
		}
	}
}

func (s *TaskScheduler) synchronize(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	fn()
}

// Close closes the TaskGroup and discards all in-flight tasks.
func (s *TaskScheduler) Close() error {
	s.once.Do(s.init)

	s.cancel()
	s.wg.Wait()
	return nil
}

// TaskGroup manages a logical group of tasks.
//
// It exposes nearly the same API as the TaskScheduler, but adds a new Poll
// method, ensures that only tasks local to the group can be retrieved or
// discarded through the same group, and automatically discards all in-flight
// tasks when closed.
type TaskGroup struct {
	scheduler   *TaskScheduler
	tasks       map[TaskID]struct{}
	completions chan TaskID
	mu          sync.Mutex
}

// NewTaskGroup creates a new task group.
func NewTaskGroup(s *TaskScheduler) *TaskGroup {
	return &TaskGroup{
		scheduler:   s,
		tasks:       map[TaskID]struct{}{},
		completions: make(chan TaskID),
	}
}

// Submit submits a task for execution.
//
// See TaskScheduler.Submit for more information.
func (g *TaskGroup) Submit(moduleSpec ModuleSpec, logSpec *LogSpec, input TaskInput, processID ProcessID) (TaskID, error) {
	taskID, err := g.scheduler.Submit(moduleSpec, logSpec, input, processID, g.completions)
	if err != nil {
		return TaskID{}, err
	}
	g.mu.Lock()
	g.tasks[taskID] = struct{}{}
	g.mu.Unlock()
	return taskID, nil
}

// Lookup looks up a task by ID.
//
// See TaskScheduler.Lookup for more information.
func (g *TaskGroup) Lookup(id TaskID) (task TaskInfo, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok = g.tasks[id]; !ok {
		return
	}

	return g.scheduler.Lookup(id)
}

// Poll returns a channel that receives completion notifications
// for tasks in the group.
func (g *TaskGroup) Poll() <-chan TaskID {
	return g.completions
}

// Discard discards a task by ID.
//
// See TaskScheduler.Discard for more information.
func (g *TaskGroup) Discard(id TaskID) (ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok = g.tasks[id]; !ok {
		return
	}
	ok = g.scheduler.Discard(id)
	delete(g.tasks, id)
	return
}

// Close closes the TaskGroup and discards all in-flight tasks.
func (g *TaskGroup) Close() error {
	g.mu.Lock()
	defer g.mu.Unlock()

	for taskID := range g.tasks {
		g.scheduler.Discard(taskID)
	}
	g.tasks = nil

	// Note: we don't close(g.completions) here in case the scheduler tries to
	// write a completion notification to the channel (which would cause a
	// panic). Just let the channel be garbage collected.

	return nil
}
