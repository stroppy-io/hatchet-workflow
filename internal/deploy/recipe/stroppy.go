package recipe

import "fmt"

// stroppy driver_type values (stroppy RunConfig).
const (
	driverPostgres = "postgres" // postgres, cockroach
	driverMySQL    = "mysql"    // mysql, mariadb
	driverPicodata = "picodata"
	driverYDB      = "ydb"
)

// stroppyConfigPath is where the workload node writes the run config.
const stroppyConfigPath = "/tmp/stroppy-config.json"

// stroppyRunSteps fetches the stroppy release tarball through the server, unpacks
// the binary, writes a stroppy RunConfig JSON (tpcc, ~1 minute k6 duration) and
// runs it — `stroppy run -f <config>`. Mirrors the old main flow (config-driven).
func stroppyRunSteps(driver, script, dsn string, refs Refs) []Step {
	unpack := `set -e
if tar tzf /tmp/stroppy.dl >/dev/null 2>&1; then
  tar xzf /tmp/stroppy.dl -C /tmp
  BIN=$(find /tmp -maxdepth 2 -type f -name stroppy | head -1)
  cp "$BIN" /usr/local/bin/stroppy
else
  cp /tmp/stroppy.dl /usr/local/bin/stroppy
fi
chmod +x /usr/local/bin/stroppy`

	return []Step{
		fetchStep("fetch stroppy", "/tmp/stroppy.dl", 0o644, refs.StroppyBinaryURL, refs.StroppyChecksum),
		cmdStep("unpack stroppy", unpack),
		writeStep("write stroppy config", stroppyConfigPath, 0o644, renderStroppyConfig(driver, script, dsn)),
		cmdStep("run stroppy", fmt.Sprintf("set -e\ncd /tmp\n/usr/local/bin/stroppy run -f %s", stroppyConfigPath)),
	}
}

// renderStroppyConfig renders a stroppy RunConfig JSON: one driver pointed at dsn,
// tpcc script, 100-conn pool, and a 60-second k6 duration load.
func renderStroppyConfig(driver, script, dsn string) string {
	return fmt.Sprintf(`{
  "version": "1",
  "script": "%s",
  "drivers": {
    "0": {
      "driverType": "%s",
      "url": "%s",
      "defaultInsertMethod": "native",
      "pool": { "maxConns": 100, "minConns": 100 }
    }
  },
  "env": { "SCALE_FACTOR": "1", "POOL_SIZE": "100" },
  "k6Args": ["-q", "--vus", "1", "--duration", "60s"],
  "global": { "logger": { "logLevel": "LOG_LEVEL_INFO" } }
}
`, script, driver, dsn)
}
