package provisioning

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	hatchetLib "github.com/hatchet-dev/hatchet/sdks/go"
	"github.com/sourcegraph/conc/pool"
	valkeygo "github.com/valkey-io/valkey-go"
)

const (
	K8SConfigPath = "K8S_CONFIG_PATH"
	ValkeyUrl     = "VALKEY_URL"
)

var ErrWorkerNotUp = errors.New("worker not up")

func valkeyFromEnv() (valkeygo.Client, error) {
	urlStr := os.Getenv(ValkeyUrl)
	if urlStr == "" {
		return nil, fmt.Errorf("environment variable %s is not set", ValkeyUrl)
	}
	valkeyUrl, err := valkeygo.ParseURL(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse Valkey URL: %w", err)
	}
	valkeyClient, err := valkeygo.NewClient(valkeyUrl)
	if err != nil {
		return nil, fmt.Errorf("failed to create Valkey client: %w", err)
	}
	return valkeyClient, nil
}

func waitMultipleWorkersUp(hctx hatchetLib.Context, c *hatchetLib.Client, names ...string) error {
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
