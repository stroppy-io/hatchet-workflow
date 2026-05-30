package expand

import "fmt"

// Provider kind tokens (match the provider schema names).
const (
	ProviderYandex = "yandex"
	ProviderDocker = "docker"
)

// Expander derives the database VMs from a baked database config (the schema's
// values, e.g. structpb.Struct.AsMap()).
type Expander func(db map[string]any) []VM

// expanders is the per-kind registry. Each database package registers its
// expander in an init().
var expanders = map[string]Expander{}

// Register wires an expander for a database kind (the schema name).
func Register(kind string, e Expander) { expanders[kind] = e }

// Expand derives the database VMs for the given kind. Returns an error if the
// kind has no registered expander.
func Expand(kind string, db map[string]any) ([]VM, error) {
	e, ok := expanders[kind]
	if !ok {
		return nil, fmt.Errorf("expand: no expander registered for %q", kind)
	}
	return e(db), nil
}

// WithWorkload appends `runners` stroppy workload VMs to the database VMs — the
// final deployment topology is database VMs ++ workload VMs.
func WithWorkload(dbVMs []VM, runners int) []VM {
	out := append([]VM(nil), dbVMs...)
	for i := 1; i <= runners; i++ {
		out = append(out, VM{Role: RoleWorkload, Name: fmt.Sprintf("stroppy-%d", i), Shape: ShapeWorkload})
	}
	return out
}

// --- map helpers (values come from structpb: numbers are float64) ---

func mapStr(m map[string]any, key, def string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return def
}

func mapInt(m map[string]any, key string, def int) int {
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return def
}

func mapBool(m map[string]any, key string, def bool) bool {
	if v, ok := m[key].(bool); ok {
		return v
	}
	return def
}

func mapObj(m map[string]any, key string) map[string]any {
	if v, ok := m[key].(map[string]any); ok {
		return v
	}
	return map[string]any{}
}

// mapStrSlice reads a []string from a structpb list ([]any of strings).
func mapStrSlice(m map[string]any, key string) []string {
	raw, ok := m[key].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, v := range raw {
		if s, ok := v.(string); ok {
			out = append(out, s)
		}
	}
	return out
}
