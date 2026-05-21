package valkey

import (
	"fmt"

	"github.com/valkey-io/valkey-go"
	"github.com/valkey-io/valkey-go/valkeyotel"
)

const (
	Dollar       = "$"
	Zero         = "0"
	Arrow        = ">"
	Star         = "*"
	DefaultBlock = 1000
	DefaultCount = 10
	ZeroBlock    = 0
)

func NewValkey(cfg *Config) (valkey.Client, error) {
	client, err := valkeyotel.NewClient(valkey.ClientOption{
		InitAddress: cfg.Addresses,
		Username:    cfg.Username,
		Password:    cfg.Password,
		ClientName:  fmt.Sprintf("valkey.client"),
	})
	if err != nil {
		return nil, err
	}
	return client, nil
}
