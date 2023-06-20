package timecraft

import (
	"context"
	"fmt"
	"sync"

	"github.com/google/uuid"
)

// TaskID is a task identifier.
type TaskID = uuid.UUID

// TaskScheduler schedules tasks.
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

type taskInfo struct {
	id  TaskID
	req HTTPRequest
}

// SubmitTask submits a task for execution.
//
// The management of WebAssembly module processes to execute tasks and the
// scheduling of tasks across processes are both implementation details.
//
// SubmitTask returns a TaskID that can be used to query task status and
// results.
func (s *TaskScheduler) SubmitTask(moduleSpec ModuleSpec, logSpec *LogSpec, req HTTPRequest) (id TaskID, err error) {
	s.once.Do(s.init)

	id = uuid.New()

	task := &taskInfo{
		id:  id,
		req: req,
	}

	s.mu.Lock()
	s.tasks[id] = task
	s.mu.Unlock()

	s.queue <- task

	return id, nil
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
