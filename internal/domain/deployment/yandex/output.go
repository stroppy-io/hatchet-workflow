package yandex

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"github.com/stroppy-io/hatchet-workflow/internal/infrastructure/terraform"
)

type VmIpsOutput = map[string]struct {
	ID         string `json:"id"`
	NatIP      string `json:"nat_ip"`
	InternalIP string `json:"internal_ip"`
}

const VmIpsOutputKey consts.ConstValue = "vm_ips"

func GetVmIpsOutput(output terraform.TfOutput) (VmIpsOutput, error) {
	return terraform.GetTfOutputVal[VmIpsOutput](output, VmIpsOutputKey)
}
