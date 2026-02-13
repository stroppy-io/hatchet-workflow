package test

import (
	"context"
	"errors"
	"fmt"
	"time"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/sourcegraph/conc/pool"
	"github.com/stroppy-io/hatchet-workflow/internal/proto/provision"
)

var ErrWorkerNotUp = errors.New("worker not up")

func waitMultipleWorkersUp(hctx hatchetLib.Context, c *hatchetLib.Client, provision *provision.DeployedPlacement) error {
	var names []string
	for _, item := range provision.GetItems() {
		if item.GetPlacementItem().GetWorker() != nil {
			names = append(names, item.GetPlacementItem().GetWorker().GetWorkerName())
		}
	}
	p := pool.New().WithContext(hctx.GetContext()).WithFailFast().WithCancelOnError().WithFirstError()
	for _, name := range names {
		p.Go(func(ctx context.Context) error {
			return waitWorkerUp(hctx, c, name)
		})
	}
	return p.Wait()
}

func waitWorkerUp(hctx hatchetLib.Context, c *hatchetLib.Client, name string) error {
	checkExists := func(ctx context.Context) bool {
		workers, err := c.Workers().List(ctx)
		if err != nil {
			return false
		}
		for _, worker := range *workers.Rows {
			hctx.Log(fmt.Sprintf("found worker: %+v", worker))
			if worker.Name == name {
				return true
			}
		}
		return false
	}
	for {
		select {
		case <-hctx.Done():
			return errors.Join(ErrWorkerNotUp, hctx.Err())
		default:
			if checkExists(hctx) {
				return nil
			}
			time.Sleep(time.Second)
		}
	}
}
