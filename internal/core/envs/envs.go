package envs

import "fmt"

func ToSlice(envs map[string]string) []string {
	var result []string
	for key, value := range envs {
		result = append(result, fmt.Sprintf("%s=%s", key, value))
	}
	return result
}
