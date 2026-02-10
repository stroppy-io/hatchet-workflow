package k8s

import (
	"github.com/stroppy-io/hatchet-workflow/internal/core/consts"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

const (
	fieldManager consts.Str = "go-apply"
	trueString   consts.Str = "True"
)

const (
	syncedCondition consts.Str = "Synced"
	readyCondition  consts.Str = "Ready"
)

const (
	statusField     consts.Str = "status"
	atProviderField consts.Str = "atProvider"
	idField         consts.Str = "id"
)

func pointer(b bool) *bool { return &b }

func getCondition(obj *unstructured.Unstructured, condType string) string {
	conds, found, err := unstructured.NestedSlice(obj.Object, "status", "conditions")
	if err != nil || !found {
		return ""
	}

	for _, c := range conds {
		m, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		t, _ := m["type"].(string)
		s, _ := m["status"].(string)
		if t == condType {
			return s
		}
	}
	return ""
}
