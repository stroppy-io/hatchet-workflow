// Package ids names every schema namespace of the product and builds
// identities the same way everywhere.
package ids

import (
	"strings"

	schemapb "github.com/gopherex/schemapb/go/schemapb"
)

// Namespaces. The public schema id is "<namespace>.<name>@<major>".
const (
	NSDB       schemapb.Namespace = "db"       // db.<kind>.params — database topology/options
	NSCfg      schemapb.Namespace = "cfg"      // cfg.<software> — a software config rendered to a file
	NSWorkload schemapb.Namespace = "workload" // workload.* — stroppy workload definitions
	NSProvider schemapb.Namespace = "provider" // provider.<kind>.settings|credentials
	NSSpec     schemapb.Namespace = "spec"     // spec.* — what the pipelines receive/return
	NSSystem   schemapb.Namespace = "system"   // system.* — platform settings
	NSTest     schemapb.Namespace = "test"     // test.* — test-level forms
	NSTenant   schemapb.Namespace = "tenant"   // tenant.* — tenant-level forms
)

// ID builds a schema identity from a major version.
func ID(ns schemapb.Namespace, name schemapb.SchemaName, major uint64) *schemapb.SchemaIdentity {
	return schemapb.ID(ns, name, schemapb.Ver(major, 0, 0))
}

// IDMinor builds a schema identity from major.minor — for software whose
// line is named that way (mysql 8.4, mariadb 10.11). Public id: "@8.4".
func IDMinor(ns schemapb.Namespace, name schemapb.SchemaName, major, minor uint64) *schemapb.SchemaIdentity {
	return schemapb.ID(ns, name, schemapb.Ver(major, minor, 0))
}

// CfgMinor is cfg.<software>@<major>.<minor>.
func CfgMinor(software string, major, minor uint64) *schemapb.SchemaIdentity {
	return IDMinor(NSCfg, schemapb.SchemaName(software), major, minor)
}

// DB is db.<kind>.params.
func DB(kind string, major uint64) *schemapb.SchemaIdentity {
	return ID(NSDB, schemapb.SchemaName(kind+".params"), major)
}

// Cfg is cfg.<software>.
func Cfg(software string, major uint64) *schemapb.SchemaIdentity {
	return ID(NSCfg, schemapb.SchemaName(software), major)
}

// Workload is workload.<name>.
func Workload(name string, major uint64) *schemapb.SchemaIdentity {
	return ID(NSWorkload, schemapb.SchemaName(name), major)
}

// Provider is provider.<kind>.<part>.
func Provider(kind, part string, major uint64) *schemapb.SchemaIdentity {
	return ID(NSProvider, schemapb.SchemaName(kind+"."+part), major)
}

// Spec is spec.<name>.
func Spec(name string, major uint64) *schemapb.SchemaIdentity {
	return ID(NSSpec, schemapb.SchemaName(name), major)
}

// System is system.<name>.
func System(name string, major uint64) *schemapb.SchemaIdentity {
	return ID(NSSystem, schemapb.SchemaName(name), major)
}

// Test is test.<name>.
func Test(name string, major uint64) *schemapb.SchemaIdentity {
	return ID(NSTest, schemapb.SchemaName(name), major)
}

// Tenant is tenant.<name>.
func Tenant(name string, major uint64) *schemapb.SchemaIdentity {
	return ID(NSTenant, schemapb.SchemaName(name), major)
}

// Public renders the public id "<ns>.<name>@<major>" (or "@<major>.<minor>"
// when the minor is non-zero) — the form used in the API (x-schema,
// /catalog/schemas/{id}) and for golden file names.
func Public(id *schemapb.SchemaIdentity) string {
	v := strings.TrimPrefix(id.GetVersion(), "v")
	parts := strings.SplitN(v, ".", 3)
	out := parts[0]
	if len(parts) > 1 && parts[1] != "0" {
		out += "." + parts[1]
	}
	return id.GetNamespace() + "." + id.GetName() + "@" + out
}
