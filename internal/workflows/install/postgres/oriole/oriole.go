package oriole

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Config holds the configuration for the OrioleDB installation.
type Config struct {
	Version         string            // e.g., "14", "15"
	Port            int               // e.g., 5432
	ListenAddresses string            // e.g., "*"
	Password        string            // Password for the 'postgres' user
	Settings        map[string]string // Additional postgresql.conf settings
}

// InstallAndConfigure installs OrioleDB, configures it, and waits for it to be healthy.
func InstallAndConfigure(ctx context.Context, cfg Config) error {
	if err := installPackage(ctx, cfg.Version); err != nil {
		return fmt.Errorf("failed to install orioledb: %w", err)
	}

	// Initialize DB if necessary (mostly for RHEL/CentOS)
	if err := initializeDB(ctx, cfg.Version); err != nil {
		return fmt.Errorf("failed to initialize database: %w", err)
	}

	if err := applyConfiguration(ctx, cfg); err != nil {
		return fmt.Errorf("failed to apply configuration: %w", err)
	}

	if err := startService(ctx, cfg.Version); err != nil {
		return fmt.Errorf("failed to start service: %w", err)
	}

	if cfg.Password != "" {
		if err := setPostgresPassword(ctx, cfg.Password); err != nil {
			return fmt.Errorf("failed to set postgres password: %w", err)
		}
	}

	if err := waitForHealth(ctx, cfg.Port); err != nil {
		return fmt.Errorf("health check failed: %w", err)
	}

	if err := createExtension(ctx, cfg.Port); err != nil {
		return fmt.Errorf("failed to create orioledb extension: %w", err)
	}

	return nil
}

func installPackage(ctx context.Context, version string) error {
	if _, err := exec.LookPath("apt-get"); err == nil {
		// Debian/Ubuntu
		// Add PostgreSQL repo
		if err := runCommand(ctx, "sh", "-c", "echo \"deb [signed-by=/usr/share/keyrings/postgresql.gpg] http://apt.postgresql.org/pub/repos/apt/ $(/usr/bin/lsb_release -cs)-pgdg main\" > /etc/apt/sources.list.d/pgdg.list"); err != nil {
			return err
		}
		if err := runCommand(ctx, "sh", "-c", "curl -fsSL https://www.postgresql.org/media/keys/ACCC4CF8.asc | gpg --dearmor -o /usr/share/keyrings/postgresql.gpg"); err != nil {
			return err
		}
		// Ensure update is run
		if err := runCommand(ctx, "apt-get", "update"); err != nil {
			return err
		}
		// OrioleDB usually provides packages named postgresql-<version>-oriole
		pkg := fmt.Sprintf("postgresql-%s-oriole", version)
		return runCommand(ctx, "apt-get", "install", "-y", pkg)
	} else if _, err := exec.LookPath("rpm"); err == nil {
		// RHEL/CentOS
		pkgMgr := "yum"
		if _, err := exec.LookPath("dnf"); err == nil {
			pkgMgr = "dnf"
		}
		// Assuming package name follows similar pattern
		pkg := fmt.Sprintf("postgresql%s-oriole", version)
		if err := runCommand(ctx, pkgMgr, "install", "-y", pkg); err != nil {
			// Fallback or retry? For now just return error
			return err
		}
		return nil
	}
	return fmt.Errorf("unsupported package manager")
}

func initializeDB(ctx context.Context, version string) error {
	// Debian/Ubuntu usually initializes automatically.
	// RHEL/CentOS requires initdb.
	if _, err := exec.LookPath("apt-get"); err == nil {
		return nil
	}

	// Check for setup script common in RHEL packages
	// Path might vary for OrioleDB packages, but often they use standard paths
	setupScript := fmt.Sprintf("/usr/pgsql-%s/bin/postgresql-%s-setup", version, version)
	if _, err := os.Stat(setupScript); err == nil {
		_ = runCommand(ctx, setupScript, "initdb")
		return nil
	}

	if _, err := exec.LookPath("postgresql-setup"); err == nil {
		_ = runCommand(ctx, "postgresql-setup", "--initdb")
	}

	return nil
}

func applyConfiguration(ctx context.Context, cfg Config) error {
	confPath, err := findPostgresConf(cfg.Version)
	if err != nil {
		return fmt.Errorf("could not locate postgresql.conf: %w", err)
	}

	f, err := os.OpenFile(confPath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := f.WriteString(fmt.Sprintf("\n# Auto-generated settings for OrioleDB\n")); err != nil {
		return err
	}

	if cfg.Port > 0 {
		if _, err := f.WriteString(fmt.Sprintf("port = %d\n", cfg.Port)); err != nil {
			return err
		}
	}

	if cfg.ListenAddresses != "" {
		if _, err := f.WriteString(fmt.Sprintf("listen_addresses = '%s'\n", cfg.ListenAddresses)); err != nil {
			return err
		}
	}

	// Ensure orioledb is in shared_preload_libraries
	orioleLib := "orioledb"
	if cfg.Settings == nil {
		cfg.Settings = make(map[string]string)
	}

	if val, ok := cfg.Settings["shared_preload_libraries"]; ok {
		if !strings.Contains(val, orioleLib) {
			cfg.Settings["shared_preload_libraries"] = val + "," + orioleLib
		}
	} else {
		cfg.Settings["shared_preload_libraries"] = orioleLib
	}

	for k, v := range cfg.Settings {
		if _, err := f.WriteString(fmt.Sprintf("%s = '%s'\n", k, v)); err != nil {
			return err
		}
	}

	return nil
}

func findPostgresConf(version string) (string, error) {
	candidates := []string{
		fmt.Sprintf("/etc/postgresql/%s/main/postgresql.conf", version), // Debian/Ubuntu
		fmt.Sprintf("/var/lib/pgsql/%s/data/postgresql.conf", version),  // RHEL PGDG
		"/var/lib/pgsql/data/postgresql.conf",                           // RHEL Default
	}

	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("postgresql.conf not found in standard locations")
}

func startService(ctx context.Context, version string) error {
	// Try systemctl first
	if _, err := exec.LookPath("systemctl"); err == nil {
		svcNames := []string{
			fmt.Sprintf("postgresql@%s-main", version),
			fmt.Sprintf("postgresql-%s", version),
			"postgresql",
		}

		for _, name := range svcNames {
			if err := runCommand(ctx, "systemctl", "enable", "--now", name); err == nil {
				return nil
			}
		}
	}

	// Fallback for non-systemd environments (like basic docker containers)
	if _, err := exec.LookPath("pg_ctlcluster"); err == nil {
		if err := runCommand(ctx, "pg_ctlcluster", version, "main", "start"); err == nil {
			return nil
		}
	}

	return fmt.Errorf("failed to start postgresql service with systemctl or pg_ctlcluster")
}

func setPostgresPassword(ctx context.Context, password string) error {
	time.Sleep(2 * time.Second)
	query := fmt.Sprintf("ALTER USER postgres PASSWORD '%s';", password)
	cmd := exec.CommandContext(ctx, "su", "-", "postgres", "-c", fmt.Sprintf("psql -c \"%s\"", query))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set password: %s, %w", string(out), err)
	}
	return nil
}

func waitForHealth(ctx context.Context, port int) error {
	if port == 0 {
		port = 5432
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	timeout := time.After(60 * time.Second)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for postgres on port %d", port)
		case <-ticker.C:
			conn, err := net.DialTimeout("tcp", fmt.Sprintf("localhost:%d", port), 500*time.Millisecond)
			if err == nil {
				conn.Close()
				if _, err := exec.LookPath("pg_isready"); err == nil {
					if err := runCommand(ctx, "pg_isready", "-p", fmt.Sprintf("%d", port)); err == nil {
						return nil
					}
				} else {
					return nil
				}
			}
		}
	}
}

func createExtension(ctx context.Context, port int) error {
	if port == 0 {
		port = 5432
	}
	query := "CREATE EXTENSION IF NOT EXISTS orioledb;"
	// Run as postgres user
	cmd := exec.CommandContext(ctx, "su", "-", "postgres", "-c", fmt.Sprintf("psql -p %d -c \"%s\"", port, query))
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to create extension: %s, %w", string(out), err)
	}
	return nil
}

func runCommand(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("cmd %s %v failed: %s: %w", name, args, string(out), err)
	}
	return nil
}
