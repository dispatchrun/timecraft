package timecraft

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// TaskScheduler schedules tasks.
//
// A task is small unit of work. A process (managed by the Executor) is
// responsible for executing one or more tasks. The management of processes to
// execute tasks and the scheduling of tasks across processes are both
// implementation details.
//
// At this time, a task is equal to one HTTP request. Additional types of
// work may be added in the future.
type TaskScheduler struct {
	Executor *Executor

	queue chan<- *taskInfo
	tasks map[TaskID]*taskInfo

	once   sync.Once
	ctx    context.Context
	cancel context.CancelFunc
	done   chan struct{}
	mu     sync.Mutex
}

// TaskID is a task identifier.
type TaskID = uuid.UUID

type taskInfo struct {
	id  TaskID
	req HTTPRequest
}

// SubmitTask submits a task for execution.
//
// The method returns a TaskID that can be used to query task status and
// results.
func (s *TaskScheduler) SubmitTask(moduleSpec ModuleSpec, logSpec *LogSpec, req HTTPRequest) (TaskID, error) {
	s.once.Do(s.init)

	task := &taskInfo{
		id:  uuid.New(),
		req: req,
	}

	s.mu.Lock()
	s.tasks[task.id] = task
	s.mu.Unlock()

	s.queue <- task

	return task.id, nil
}

func (s *TaskScheduler) init() {
	s.tasks = map[TaskID]*taskInfo{}

	s.ctx, s.cancel = context.WithCancel(context.Background())

	s.done = make(chan struct{})

	queue := make(chan *taskInfo)
	s.queue = queue
	go s.scheduleLoop(queue)
}

func (s *TaskScheduler) scheduleLoop(queue <-chan *taskInfo) {
	for {
		select {
		case <-s.ctx.Done():
			close(s.done)
			return
		case task := <-queue:
			s.scheduleTask(task)
		}
	}
}

func (s *TaskScheduler) scheduleTask(task *taskInfo) {
	fmt.Println("scheduling task", task.id)
}

func (s *TaskScheduler) Close() error {
	s.once.Do(s.init)

	s.cancel()
	<-s.done
	return nil
}
