package recipe

import (
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/common"
)

// textFile builds a BakedFile wrapping a plain text common.File at path with the
// given content and mode. For the demo the File is wrapped directly in a
// BakedFile (no resolver/baked data).
func textFile(path string, mode uint32, content string) *common.BakedFile {
	return &common.BakedFile{
		File: &common.File{
			Info:    &common.File_Info{Path: path, Mode: mode},
			Content: &common.File_Text{Text: content},
		},
	}
}

// scriptCmd builds a shell-script Cmd. The script runs through /bin/bash.
func scriptCmd(text string) *common.Cmd {
	return &common.Cmd{
		Spec: &common.Cmd_Spec{
			Command: &common.Cmd_Spec_Script{
				Script: &common.Cmd_Script{Text: text, Shell: "/bin/bash"},
			},
		},
	}
}
