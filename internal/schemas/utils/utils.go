// Package utils holds schema-authoring helpers shared across every schema
// package (databases, providers, machine, cluster). Anything generic and
// schema-agnostic lives here so each schema package stays focused on its own
// field vocabulary.
package utils

import (
	"fmt"

	"github.com/stroppy-io/schemapb/schemapb"
)

// StrEnum builds a string field constrained to a fixed option set. Choice fields
// are modelled as strings (not int enums) so expr-lang gates read naturally
// (root.x == 'token') and the stored value IS the config token used downstream.
func StrEnum(name string, opts ...string) *schemapb.StrB {
	return schemapb.Str(name).In(opts...)
}

// Eq builds the expr `path == 'val'` (string literal).
func Eq(path, val string) string { return fmt.Sprintf("%s == '%s'", path, val) }

// Ne builds the expr `path != 'val'` (string literal).
func Ne(path, val string) string { return fmt.Sprintf("%s != '%s'", path, val) }

// Gte builds the expr `path >= n`.
func Gte(path string, n int) string { return fmt.Sprintf("%s >= %d", path, n) }

// Lte builds the expr `path <= n`.
func Lte(path string, n int) string { return fmt.Sprintf("%s <= %d", path, n) }

// IsTrue builds the expr `path == true`.
func IsTrue(path string) string { return path + " == true" }
