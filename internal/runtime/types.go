package runtime

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/primitive"
	"github.com/stroppy-io/stroppy-cloud/internal/utils/protohelp"
)

type Task[I, O proto.Message] interface {
	Call(I) (O, error)
	Name() string
}

type TaskFn[I, O proto.Message] func(I) (O, error)

func (t TaskFn[I, O]) Call(i I) (O, error) {
	return t(i)
}

type wrapper[I, O proto.Message] struct {
	name         string
	downstreamFn TaskFn[I, O]
}

func (w wrapper[I, O]) Call(i *anypb.Any) (*anypb.Any, error) {
	input := protohelp.ProtoNew[I]()
	err := i.UnmarshalTo(input)
	if err != nil {
		return nil, err
	}
	output, err := w.downstreamFn(input)
	if err != nil {
		return nil, err
	}
	return anypb.New(output)
}

func (w wrapper[I, O]) Name() string {
	return w.name
}

func NewTask[I, O proto.Message](name string, handler TaskFn[I, O]) Task[*anypb.Any, *anypb.Any] {
	return &wrapper[I, O]{
		name:         name,
		downstreamFn: handler,
	}
}

type TasksRegistry interface {
	GetTaskHandler(name string) (Task[*anypb.Any, *anypb.Any], bool)
}

type TaskRegistry struct {
	tasks map[string]Task[*anypb.Any, *anypb.Any]
}

func NewTaskRegistry(tasks ...Task[*anypb.Any, *anypb.Any]) *TaskRegistry {
	r := &TaskRegistry{tasks: make(map[string]Task[*anypb.Any, *anypb.Any], len(tasks))}
	for _, task := range tasks {
		r.Register(task)
	}
	return r
}

func (r *TaskRegistry) Register(task Task[*anypb.Any, *anypb.Any]) {
	if r.tasks == nil {
		r.tasks = make(map[string]Task[*anypb.Any, *anypb.Any])
	}
	r.tasks[task.Name()] = task
}

func (r *TaskRegistry) GetTaskHandler(name string) (Task[*anypb.Any, *anypb.Any], bool) {
	if r == nil {
		return nil, false
	}
	task, ok := r.tasks[name]
	return task, ok
}

func RunTask(state *primitive.Dag_Node_TaskState, tasksReg TasksRegistry) (*primitive.Dag_Node_TaskState, error) {
	task, ok := tasksReg.GetTaskHandler(state.GetHandlerName())
	if !ok {
		return nil, NewFailureError(fmt.Errorf("task handler %q not found", state.GetHandlerName()), &primitive.Dag_Failure{
			Message: fmt.Sprintf("task handler %q not found", state.GetHandlerName()),
			Code:    FailureCodeTaskHandlerNotFound,
			Source:  FailureSourceRuntime,
			Phase:   FailurePhaseTaskLookup,
		})
	}
	output, err := task.Call(state.GetInput())
	if err != nil {
		return nil, err
	}
	newState := &primitive.Dag_Node_TaskState{
		HandlerName: state.GetHandlerName(),
		Input:       state.GetInput(),
		Output:      output,
	}
	return newState, nil
}

const (
	FailureSourceRuntime = "runtime"
	FailureSourceTask    = "task"

	FailurePhaseTaskLookup = "task_lookup"
	FailurePhaseTaskCall   = "task_call"
	FailurePhaseDag        = "dag"
	FailurePhaseSubDag     = "sub_dag"
	FailurePhaseDagRef     = "dag_ref"
	FailurePhaseRecovery   = "recovery"

	FailureCodeTaskHandlerNotFound = "TASK_HANDLER_NOT_FOUND"
	FailureCodeTaskFailed          = "TASK_FAILED"
	FailureCodeSubDagFailed        = "SUB_DAG_FAILED"
	FailureCodeDagRefFailed        = "DAG_REF_FAILED"
	FailureCodeDagRefPending       = "DAG_REF_PENDING"
	FailureCodeDagRefRunnerMissing = "DAG_REF_RUNNER_MISSING"
	FailureCodeDagInvalid          = "DAG_INVALID"
	FailureCodeDagCancelled        = "DAG_CANCELLED"
	FailureCodeNodeInterrupted     = "NODE_INTERRUPTED"
)

type FailureError struct {
	err     error
	failure *primitive.Dag_Failure
}

func NewFailureError(err error, failure *primitive.Dag_Failure) *FailureError {
	if err == nil {
		err = fmt.Errorf("runtime failure")
	}
	if failure == nil {
		failure = &primitive.Dag_Failure{}
	}
	if failure.Message == "" {
		failure.Message = err.Error()
	}
	return &FailureError{err: err, failure: failure}
}

func (e *FailureError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *FailureError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func (e *FailureError) Failure() *primitive.Dag_Failure {
	if e == nil {
		return nil
	}
	return proto.Clone(e.failure).(*primitive.Dag_Failure)
}

// errorRetryableOpinion reports the executor/task opinion on whether an error
// is retryable, independent of retry policy budget. A structured *FailureError
// carries the opinion in its Failure.retryable; a plain error is assumed
// transient (retryable) until proven otherwise.
func errorRetryableOpinion(err error) bool {
	var fe *FailureError
	if errors.As(err, &fe) {
		return fe.Failure().GetRetryable()
	}
	return true
}

func FailureFromError(err error, source, phase, code string, attempt uint32, retryable bool, metadata map[string]string) *primitive.Dag_Failure {
	if err == nil {
		return nil
	}
	var fe *FailureError
	if ok := errors.As(err, &fe); ok {
		failure := fe.Failure()
		if failure.Source == "" {
			failure.Source = source
		}
		if failure.Phase == "" {
			failure.Phase = phase
		}
		if failure.Code == "" {
			failure.Code = code
		}
		if failure.Attempt == 0 {
			failure.Attempt = attempt
		}
		failure.Retryable = retryable
		if failure.OccurredAt == nil {
			failure.OccurredAt = timestamppb.Now()
		}
		if len(metadata) > 0 {
			if failure.Metadata == nil {
				failure.Metadata = make(map[string]string, len(metadata))
			}
			for k, v := range metadata {
				if _, exists := failure.Metadata[k]; !exists {
					failure.Metadata[k] = v
				}
			}
		}
		return failure
	}
	return &primitive.Dag_Failure{
		Message:    err.Error(),
		Code:       code,
		Source:     source,
		Phase:      phase,
		Attempt:    attempt,
		Retryable:  retryable,
		OccurredAt: timestamppb.New(time.Now()),
		Metadata:   metadata,
	}
}
