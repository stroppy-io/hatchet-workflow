package provider

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/stroppy-io/stroppy-cloud/internal/infrastructure/terraform"
)

func TestYandexModuleDirResolver_PerRunWdId(t *testing.T) {
	tfFiles := []terraform.TfFile{terraform.NewTfFile([]byte("module {}"), "main.tf")}
	resolve := YandexModuleDirResolver(tfFiles)

	dirA, filesA, okA := resolve("tenant-1", "run-a", "yandex")
	require.True(t, okA)
	require.Equal(t, tfFiles, filesA)

	dirB, _, okB := resolve("tenant-1", "run-b", "yandex")
	require.True(t, okB)

	require.NotEqual(t, dirA, dirB, "two runs against yandex must not collide on the same terraform WdId")
	require.Equal(t, "yandex-run-a", dirA)
	require.Equal(t, "yandex-run-b", dirB)
}

func TestYandexModuleDirResolver_EmptyRunID_FallsBackToConstant(t *testing.T) {
	resolve := YandexModuleDirResolver(nil)
	dir, _, ok := resolve("tenant-1", "", "yandex")
	require.True(t, ok)
	require.Equal(t, "yandex", dir, "ad-hoc callers without a run id (tests, direct use) keep the pre-F4 constant id")
}

func TestYandexModuleDirResolver_UnknownName_NotOK(t *testing.T) {
	resolve := YandexModuleDirResolver(nil)
	_, _, ok := resolve("tenant-1", "run-a", "nope")
	require.False(t, ok)
}
