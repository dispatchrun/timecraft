package timecraft

import "net/http"

// TaskID is a task identifier.
type TaskID string

// TaskRequest is a request to the timecraft runtime asking it to
// schedule a task for execution.
type TaskRequest struct {
	// Module is details about the WebAssembly module that's responsible
	// for executing the task.
	Module ModuleSpec

	// Input is input to the task.
	Input TaskInput
}

// TaskResponse is information about a task from the timecraft runtime.
type TaskResponse struct {
	// ID is the task identifier.
	ID TaskID

	// State is the current state of the task.
	State TaskState

	// Error is the error that occurred during task initialization or execution
	// (if applicable).
	Error error

	// Output is the output of the task, if it executed successfully.
	Output TaskOutput

	// ProcessID is the identifier of the process that handled the task
	// (if applicable).
	ProcessID ProcessID
}

// ProcessID is a process identifier.
type ProcessID string

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

// TaskInput is input to a task.
type TaskInput interface{ taskInput() }

// HTTPRequest is an HTTP request.
type HTTPRequest struct {
	Method  string
	Path    string
	Headers http.Header
	Body    []byte
}

func (*HTTPRequest) taskInput() {}

// TaskOutput is output from a task.
type TaskOutput interface{ taskOutput() }

// HTTPResponse is an HTTP response.
type HTTPResponse struct {
	StatusCode int
	Headers    http.Header
	Body       []byte
}

func (*HTTPResponse) taskOutput() {}

// ModuleSpec is a WebAssembly module specification.
type ModuleSpec struct {
	Path string
	Args []string
	Env  []string
}
