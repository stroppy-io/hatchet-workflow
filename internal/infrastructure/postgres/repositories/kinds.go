package repositories

import (
	"github.com/stroppy-io/stroppy-cloud/internal/domain/catalog"
)

func libraryKind(s string) catalog.DatabaseKind { return catalog.DatabaseKind(s) }
func libraryProtocol(s string) catalog.Protocol { return catalog.Protocol(s) }
