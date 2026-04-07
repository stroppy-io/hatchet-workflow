package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func cloudUploadCmd() *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a .deb or .rpm package to the server",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			f, err := os.Open(filePath)
			if err != nil {
				return fmt.Errorf("open file: %w", err)
			}
			defer f.Close()

			ext := filepath.Ext(filePath)
			var endpoint string
			switch ext {
			case ".deb":
				endpoint = "/api/v1/upload/deb"
			case ".rpm":
				endpoint = "/api/v1/upload/rpm"
			default:
				return fmt.Errorf("unsupported file type %q (must be .deb or .rpm)", ext)
			}

			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)
			part, err := writer.CreateFormFile("file", filepath.Base(filePath))
			if err != nil {
				return err
			}
			if _, err := io.Copy(part, f); err != nil {
				return err
			}
			writer.Close()

			req, err := http.NewRequest("POST", c.url(endpoint), body)
			if err != nil {
				return err
			}
			req.Header.Set("Content-Type", writer.FormDataContentType())

			resp, err := c.do(req)
			if err != nil {
				return fmt.Errorf("upload request: %w", err)
			}
			defer resp.Body.Close()

			respBody, _ := io.ReadAll(resp.Body)
			if resp.StatusCode != 200 {
				return fmt.Errorf("upload failed %d: %s", resp.StatusCode, string(respBody))
			}

			var result struct {
				Filename string `json:"filename"`
				URL      string `json:"url"`
				Size     string `json:"size"`
			}
			json.Unmarshal(respBody, &result)
			fmt.Printf("Uploaded: %s\n  URL: %s\n  Size: %s bytes\n", result.Filename, result.URL, result.Size)
			return nil
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "path to .deb or .rpm file")
	cmd.MarkFlagRequired("file")
	return cmd
}
