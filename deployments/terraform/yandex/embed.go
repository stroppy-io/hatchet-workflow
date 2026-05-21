package yandex

import (
	"embed"

	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/runtime/system"
)

//go:embed *.tf
var tfFilesEmbed embed.FS

// EmbeddedFiles returns the embedded Yandex Terraform module as system.File
// entries (workdir-relative path + inline text content), ready to drop into
// ops.TfOperation.Input.files.
func EmbeddedFiles() ([]*system.File, error) {
	entries, err := tfFilesEmbed.ReadDir(".")
	if err != nil {
		return nil, err
	}
	files := make([]*system.File, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		content, err := tfFilesEmbed.ReadFile(e.Name())
		if err != nil {
			return nil, err
		}
		files = append(files, &system.File{
			Info: &system.File_Info{Path: e.Name()},
			Source: &system.File_Content_{Content: &system.File_Content{
				Content: &system.File_Content_Text{Text: string(content)},
			}},
		})
	}
	return files, nil
}
