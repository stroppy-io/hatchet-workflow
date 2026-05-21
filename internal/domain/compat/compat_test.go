package compat

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestEngineRecipeMatrixComplete(t *testing.T) {
	// Every supported cell resolves to a usable recipe (something to install/start).
	for _, ev := range Supported() {
		r, ok := EngineRecipe(ev.Engine, ev.Version)
		require.Truef(t, ok, "%s/%s missing", ev.Engine, ev.Version)
		usable := len(r.PreInstall) > 0 || len(r.AptPackages) > 0 || r.StartScript != ""
		require.Truef(t, usable, "%s/%s recipe is empty", ev.Engine, ev.Version)
		// apt engines name a service; binary engines a start script (ydb starts via
		// render command items, so it has neither — allow that one).
		if ev.Engine != "ydb" {
			require.Truef(t, r.ServiceName != "" || r.StartScript != "",
				"%s/%s has no service or start script", ev.Engine, ev.Version)
		}
	}
}

func TestEngineRecipeDefaultVersion(t *testing.T) {
	for _, eng := range []string{"postgres", "mysql", "mariadb", "picodata", "cockroach", "ydb"} {
		require.NotEmptyf(t, DefaultVersion(eng), "%s default version", eng)
		// "" version falls back to the engine default cell.
		def, ok := EngineRecipe(eng, "")
		require.Truef(t, ok, "%s default recipe", eng)
		pinned, ok2 := EngineRecipe(eng, DefaultVersion(eng))
		require.True(t, ok2)
		require.Equal(t, pinned.ServiceName, def.ServiceName, "%s default mismatch", eng)
	}
}

func TestEngineRecipeUnknown(t *testing.T) {
	_, ok := EngineRecipe("oracle", "23")
	require.False(t, ok, "unknown engine must not resolve")
	require.Empty(t, DefaultVersion("oracle"))
}

func TestPreInstallRefreshesAptLists(t *testing.T) {
	// Every apt-based recipe must refresh package lists first (minimal base image).
	for _, ev := range Supported() {
		r, _ := EngineRecipe(ev.Engine, ev.Version)
		if len(r.AptPackages) == 0 {
			continue
		}
		joined := strings.Join(r.PreInstall, "\n")
		require.Containsf(t, joined, "apt-get update", "%s/%s apt recipe never updates lists", ev.Engine, ev.Version)
	}
}

func TestComponentRecipes(t *testing.T) {
	require.Equal(t, "etcd", Etcd.ServiceName)
	require.Equal(t, "haproxy", HAProxy.ServiceName)
	require.Equal(t, "proxysql", ProxySQL.ServiceName)
	require.Equal(t, "prometheus-node-exporter", Monitor.ServiceName)
	for _, r := range []Recipe{Etcd, HAProxy, ProxySQL, Monitor} {
		require.Contains(t, strings.Join(r.PreInstall, "\n"), "apt-get update")
	}
}
