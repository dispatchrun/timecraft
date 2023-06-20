package timecraft

import (
	"fmt"

	"github.com/google/uuid"
)

// TaskScheduler schedules tasks.
type TaskScheduler struct {
	Executor *Executor
}

// TaskID is a task identifier.
type TaskID = uuid.UUID

// SubmitTask submits a task for execution.
//
// The management of WebAssembly module processes to execute tasks and the
// scheduling of tasks across processes are both implementation details.
//
// SubmitTask returns a TaskID that can be used to query task status and
// results.
func (s *TaskScheduler) SubmitTask(moduleSpec ModuleSpec, logSpec *LogSpec, req HTTPRequest) (TaskID, error) {
	fmt.Printf("submitting task %#v %#v %#v\n", moduleSpec, logSpec, req)
	return uuid.New(), nil
}
