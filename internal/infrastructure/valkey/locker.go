package valkey

import (
	"time"

	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeylock"
)

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
