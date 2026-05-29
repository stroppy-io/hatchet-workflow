package yandex

import (
	"embed"
)

//go:embed *.tf
var tfFilesEmbed embed.FS
