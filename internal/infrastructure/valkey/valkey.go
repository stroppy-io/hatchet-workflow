package valkey

import (
	"context"
	"fmt"
	"time"

	"github.com/gopherex/xlog"
	"github.com/gopherex/xprobe"
	"github.com/stroppy-io/stroppy-cloud/internal/old/core/build"
	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeylock"
	"github.com/valkey-io/valkey-go/valkeyotel"
)

// Mode determines how the client connects to Valkey.
type Mode string

const (
	ModeStandalone Mode = "standalone"
	ModeCluster    Mode = "cluster"
	ModeSentinel   Mode = "sentinel"
)

// Config holds Valkey connection settings.
//
// Standalone (single node, default):
//
//	valkey:
//	  addresses: ["localhost:6379"]
//	  password: "secret"
//
// Cluster (sharded, auto-discovers topology from seed nodes):
//
//	valkey:
//	  mode: cluster
//	  addresses: ["node1:6379", "node2:6379", "node3:6379"]
//	  password: "secret"
//
// Sentinel (HA with automatic failover):
//
//	valkey:
//	  mode: sentinel
//	  addresses: ["sentinel1:26379", "sentinel2:26379", "sentinel3:26379"]
//	  password: "data-node-password"
//	  sentinel_master_set: "mymaster"
//	  sentinel_username: "sentinel-user"       # optional
//	  sentinel_password: "sentinel-password"   # optional
type Config struct {
	Mode      Mode     `mapstructure:"mode"` // standalone (default), cluster, sentinel
	Addresses []string `mapstructure:"addresses" validate:"required,dive,hostname_port"`
	Username  string   `mapstructure:"username"`
	Password  string   `mapstructure:"password" validate:"required"`

	// Sentinel-specific settings.
	SentinelMasterSet string `mapstructure:"sentinel_master_set"`
	SentinelUsername  string `mapstructure:"sentinel_username"`
	SentinelPassword  string `mapstructure:"sentinel_password"`
}

func NewValkey(cfg *Config, logger *xlog.Logger) (valkey.Client, xprobe.Probe, error) {
	opt := valkey.ClientOption{
		InitAddress: cfg.Addresses,
		Username:    cfg.Username,
		Password:    cfg.Password,
		ClientName:  fmt.Sprintf("naukograd.%s.valkey.client", build.ServiceName),
	}

	switch cfg.Mode {
	case ModeSentinel:
		if cfg.SentinelMasterSet == "" {
			return nil, nil, fmt.Errorf("valkey: sentinel_master_set is required in sentinel mode")
		}
		opt.Sentinel = valkey.SentinelOption{
			MasterSet:  cfg.SentinelMasterSet,
			Username:   cfg.SentinelUsername,
			Password:   cfg.SentinelPassword,
			ClientName: fmt.Sprintf("naukograd.%s.valkey.sentinel", build.ServiceName),
		}
	case ModeCluster:
		// valkey-go auto-discovers cluster topology from InitAddress; nothing extra needed.
	case ModeStandalone, "":
		// Single-node mode — default behavior.
	default:
		return nil, nil, fmt.Errorf("valkey: unknown mode %q (expected standalone, cluster, or sentinel)", cfg.Mode)
	}

	client, err := valkeyotel.NewClient(opt)
	if err != nil {
		return nil, nil, err
	}
	return client, xprobe.FromError(func(ctx context.Context) error {
		logger.Trace("Pinging Valkey server")
		return client.Do(ctx, client.B().Ping().Build()).Error()
	}), nil
}

func NewValkeyLocker(client valkey.Client, keyValidity time.Duration) (valkeylock.Locker, error) {
	lock, err := valkeylock.NewLocker(valkeylock.LockerOption{
		ClientBuilder: func(option valkey.ClientOption) (valkey.Client, error) {
			return client, nil
		},
		KeyPrefix:      "valkeylock:",
		KeyValidity:    keyValidity,
		ExtendInterval: time.Minute,
		TryNextAfter:   time.Second,
		//NoLoopTracking: true, // enabled if all your valkey nodes >= 7.0.5
		//KeyMajority:    2,
		//FallbackSETPX:  true, // for compatibility with Valkey < 6.2
	})
	if err != nil {
		return nil, err
	}
	return lock, nil
}
