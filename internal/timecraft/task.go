package timecraft

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/google/uuid"
)

// TaskScheduler schedules tasks.
//
// A task is a unit of work. A process (managed by the Executor) is
// responsible for executing one or more tasks. The management of processes to
// execute tasks and the scheduling of tasks across processes are both
// implementation details.
type TaskScheduler struct {
	Executor *Executor

	tasks       map[TaskID]*TaskInfo
	submissions chan<- *TaskInfo
	completions chan TaskID

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

	// Success indicates that the task executed successfully. This is a terminal
	// status.
	Success
)

// TaskInfo is information about a task.
type TaskInfo struct {
	id         TaskID
	createdAt  time.Time
	state      TaskState
	processID  ProcessID
	moduleSpec ModuleSpec
	logSpec    *LogSpec
	input      TaskInput
	output     TaskOutput
	err        error
}

// TaskInput is input for a task.
type TaskInput interface {
	taskInput()
}

// TaskOutput is output from a task.
type TaskOutput interface {
	taskOutput()
}

type processKey struct {
	path string
}

// SubmitTask submits a task for execution.
//
// The method returns a TaskID that can be used to query task status and
// results.
func (s *TaskScheduler) SubmitTask(moduleSpec ModuleSpec, logSpec *LogSpec, input TaskInput) (TaskID, error) {
	s.once.Do(s.init)

	task := &TaskInfo{
		id:         uuid.New(),
		createdAt:  time.Now(),
		state:      Queued,
		moduleSpec: moduleSpec,
		logSpec:    logSpec,
		input:      input,
	}

	s.synchronize(func() {
		s.tasks[task.id] = task
	})

	s.submissions <- task

	return task.id, nil
}

func (s *TaskScheduler) init() {
	s.tasks = map[TaskID]*TaskInfo{}
	s.processes = map[processKey]ProcessID{}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	queue := make(chan *TaskInfo)
	s.submissions = queue
	s.completions = make(chan TaskID)

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

	var process ProcessInfo
	var processID ProcessID
	var ok bool
	var err error

	s.synchronize(func() {
		task.state = Initializing
		processID, ok = s.processes[key]
	})
	if ok {
		// Check that the process is still alive.
		process, ok = s.Executor.Lookup(processID)
	}

	if !ok {
		// TODO: use singleflight to initialize the process
		processID, err = s.Executor.Start(task.moduleSpec, task.logSpec)
		if err != nil {
			s.completeTask(task, err, nil)
			return
		}
		process, ok = s.Executor.Lookup(processID)
		if !ok {
			s.completeTask(task, errors.New("failed to start process"), nil)
			return
		}
		s.synchronize(func() {
			s.processes[key] = processID
		})
	}

	s.synchronize(func() {
		task.state = Executing
		task.processID = processID
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
		// TODO: timeout
	}

	res, err := client.Do(&http.Request{
		Method: request.Method,
		URL:    &url.URL{Scheme: "http", Host: "timecraft", Path: request.Path},
		Header: request.Headers,
		Body:   io.NopCloser(bytes.NewReader(request.Body)),
	})
	if err != nil {
		s.completeTask(task, err, nil)
		return
	}

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

	s.completions <- task.id
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

// Poll returns a channel that receives task IDs that are complete.
func (s *TaskScheduler) Poll() <-chan TaskID {
	return s.completions
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
