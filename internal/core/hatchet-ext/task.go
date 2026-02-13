package hatchet_ext

import (
	"github.com/hatchet-dev/hatchet/pkg/client/create"
	hatchet "github.com/hatchet-dev/hatchet/sdks/go"
)

type WorkflowName = string

type TaskName = string

type Task[WI, O any] = func(ctx hatchet.Context, input WI) (O, error)

func WTask[WI, O any](f Task[*WI, *O]) Task[WI, O] {
	return func(ctx hatchet.Context, input WI) (out O, err error) {
		res, err := f(ctx, &input)
		if err != nil {
			return out, err
		}
		return *res, nil
	}
}

type pTask[PO, I, O any] = func(ctx hatchet.Context, input I, parentOutput PO) (O, error)

func PTask[PO, WI, O any](parent create.NamedTask, f pTask[*PO, *WI, *O]) Task[WI, O] {
	return func(ctx hatchet.Context, input WI) (out O, err error) {
		var parentOutput PO
		err = ctx.ParentOutput(parent, &parentOutput)
		if err != nil {
			return out, err
		}
		res, err := f(ctx, &input, &parentOutput)
		if err != nil {
			return out, err
		}
		return *res, nil
	}
}

type pTask2[PO, PO2, I, O any] = func(ctx hatchet.Context, input I, parentOutput PO, parent2Output PO2) (O, error)

func PTask2[PO, PO2, WI, O any](parent, parent2 create.NamedTask, f pTask2[*PO, *PO2, *WI, *O]) Task[WI, O] {
	return func(ctx hatchet.Context, input WI) (out O, err error) {
		var parentOutput PO
		err = ctx.ParentOutput(parent, &parentOutput)
		if err != nil {
			return out, err
		}
		var parent2Output PO2
		err = ctx.ParentOutput(parent2, &parent2Output)
		if err != nil {
			return out, err
		}
		res, err := f(ctx, &input, &parentOutput, &parent2Output)
		if err != nil {
			return out, err
		}
		return *res, nil
	}
}
