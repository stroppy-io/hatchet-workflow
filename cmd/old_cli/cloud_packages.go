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
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
)

func cloudPackagesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "packages", Short: "Manage database packages"}
	cmd.AddCommand(pkgListCmd(), pkgGetCmd(), pkgCreateCmd(), pkgUpdateCmd(), pkgDeleteCmd(), pkgCloneCmd(), pkgUploadDebCmd())
	return cmd
}

func pkgListCmd() *cobra.Command {
	var dbKind, dbVersion string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List database packages",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			path := "/api/v1/packages"
			var params []string
			if dbKind != "" {
				params = append(params, "db_kind="+dbKind)
			}
			if dbVersion != "" {
				params = append(params, "db_version="+dbVersion)
			}
			if len(params) > 0 {
				path += "?" + strings.Join(params, "&")
			}

			data, status, err := c.doJSON("GET", path, nil)
			if err != nil {
				return err
			}
			if status != 200 {
				return fmt.Errorf("server error %d: %s", status, string(data))
			}

			var pkgs []struct {
				ID        string `json:"id"`
				Name      string `json:"name"`
				DBKind    string `json:"db_kind"`
				DBVersion string `json:"db_version"`
				BuiltIn   bool   `json:"built_in"`
				DebFile   string `json:"deb_file"`
			}
			if err := json.Unmarshal(data, &pkgs); err != nil {
				return fmt.Errorf("parse response: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tDB\tVER\tBUILTIN\tDEB")
			for _, p := range pkgs {
				builtin := "no"
				if p.BuiltIn {
					builtin = "yes"
				}
				deb := "-"
				if p.DebFile != "" {
					deb = p.DebFile
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n", p.ID, p.Name, p.DBKind, p.DBVersion, builtin, deb)
			}
			w.Flush()
			return nil
		},
	}
	cmd.Flags().StringVar(&dbKind, "db-kind", "", "filter by database kind")
	cmd.Flags().StringVar(&dbVersion, "db-version", "", "filter by database version")
	return cmd
}

func pkgGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get [package-id]",
		Short: "Show package details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			data, status, err := c.doJSON("GET", "/api/v1/packages/"+args[0], nil)
			if err != nil {
				return err
			}
			if status == 404 {
				return fmt.Errorf("package %q not found", args[0])
			}
			if status != 200 {
				return fmt.Errorf("server error %d: %s", status, string(data))
			}

			var pretty bytes.Buffer
			if err := json.Indent(&pretty, data, "", "  "); err != nil {
				fmt.Println(string(data))
				return nil
			}
			fmt.Println(pretty.String())
			return nil
		},
	}
}

func pkgCreateCmd() *cobra.Command {
	var (
		name          string
		description   string
		dbKind        string
		dbVersion     string
		aptPkgs       []string
		preInstall    []string
		customRepo    string
		customRepoKey string
		debFile       string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a custom package",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			body := map[string]any{
				"name":    name,
				"db_kind": dbKind,
			}
			if description != "" {
				body["description"] = description
			}
			if dbVersion != "" {
				body["db_version"] = dbVersion
			}
			if len(aptPkgs) > 0 {
				body["apt_packages"] = aptPkgs
			}
			if len(preInstall) > 0 {
				body["pre_install"] = preInstall
			}
			if customRepo != "" {
				body["custom_repo"] = customRepo
			}
			if customRepoKey != "" {
				body["custom_repo_key"] = customRepoKey
			}

			raw, _ := json.Marshal(body)
			data, status, err := c.doJSON("POST", "/api/v1/packages", bytes.NewReader(raw))
			if err != nil {
				return err
			}
			if status != 201 {
				return fmt.Errorf("create failed %d: %s", status, string(data))
			}

			var result struct {
				ID string `json:"id"`
			}
			json.Unmarshal(data, &result)
			fmt.Printf("Created package: %s\n", result.ID)

			if debFile != "" {
				if err := uploadDebToPackage(c, result.ID, debFile); err != nil {
					return err
				}
			}

			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "package name (required)")
	cmd.Flags().StringVar(&dbKind, "db-kind", "", "database kind (required)")
	cmd.Flags().StringVar(&description, "description", "", "package description")
	cmd.Flags().StringVar(&dbVersion, "db-version", "", "database version")
	cmd.Flags().StringSliceVar(&aptPkgs, "apt-pkg", nil, "apt packages to install")
	cmd.Flags().StringSliceVar(&preInstall, "pre-install", nil, "pre-install commands")
	cmd.Flags().StringVar(&customRepo, "custom-repo", "", "custom APT repository")
	cmd.Flags().StringVar(&customRepoKey, "custom-repo-key", "", "custom repository GPG key URL")
	cmd.Flags().StringVar(&debFile, "deb", "", "path to .deb file to upload after creation")
	cmd.MarkFlagRequired("name")
	cmd.MarkFlagRequired("db-kind")
	return cmd
}

func pkgUpdateCmd() *cobra.Command {
	var (
		name          string
		description   string
		dbKind        string
		dbVersion     string
		aptPkgs       []string
		preInstall    []string
		customRepo    string
		customRepoKey string
	)
	cmd := &cobra.Command{
		Use:   "update [package-id]",
		Short: "Update a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			body := map[string]any{}
			if cmd.Flags().Changed("name") {
				body["name"] = name
			}
			if cmd.Flags().Changed("description") {
				body["description"] = description
			}
			if cmd.Flags().Changed("db-kind") {
				body["db_kind"] = dbKind
			}
			if cmd.Flags().Changed("db-version") {
				body["db_version"] = dbVersion
			}
			if cmd.Flags().Changed("apt-pkg") {
				body["apt_packages"] = aptPkgs
			}
			if cmd.Flags().Changed("pre-install") {
				body["pre_install"] = preInstall
			}
			if cmd.Flags().Changed("custom-repo") {
				body["custom_repo"] = customRepo
			}
			if cmd.Flags().Changed("custom-repo-key") {
				body["custom_repo_key"] = customRepoKey
			}

			raw, _ := json.Marshal(body)
			data, status, err := c.doJSON("PUT", "/api/v1/packages/"+args[0], bytes.NewReader(raw))
			if err != nil {
				return err
			}
			if status == 403 {
				return fmt.Errorf("cannot edit built-in package")
			}
			if status != 200 {
				return fmt.Errorf("update failed %d: %s", status, string(data))
			}

			fmt.Println("Package updated.")
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "package name")
	cmd.Flags().StringVar(&dbKind, "db-kind", "", "database kind")
	cmd.Flags().StringVar(&description, "description", "", "package description")
	cmd.Flags().StringVar(&dbVersion, "db-version", "", "database version")
	cmd.Flags().StringSliceVar(&aptPkgs, "apt-pkg", nil, "apt packages to install")
	cmd.Flags().StringSliceVar(&preInstall, "pre-install", nil, "pre-install commands")
	cmd.Flags().StringVar(&customRepo, "custom-repo", "", "custom APT repository")
	cmd.Flags().StringVar(&customRepoKey, "custom-repo-key", "", "custom repository GPG key URL")
	return cmd
}

func pkgDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete [package-id]",
		Short: "Delete a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			data, status, err := c.doJSON("DELETE", "/api/v1/packages/"+args[0], nil)
			if err != nil {
				return err
			}
			if status != 200 {
				return fmt.Errorf("delete failed %d: %s", status, string(data))
			}

			fmt.Println("Package deleted.")
			return nil
		},
	}
}

func pkgCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone [package-id]",
		Short: "Clone a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}

			data, status, err := c.doJSON("POST", "/api/v1/packages/"+args[0]+"/clone", nil)
			if err != nil {
				return err
			}
			if status != 201 {
				return fmt.Errorf("clone failed %d: %s", status, string(data))
			}

			var result struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			}
			json.Unmarshal(data, &result)
			fmt.Printf("Cloned package: %s (id: %s)\n", result.Name, result.ID)
			return nil
		},
	}
}

func pkgUploadDebCmd() *cobra.Command {
	var filePath string
	cmd := &cobra.Command{
		Use:   "upload-deb [package-id]",
		Short: "Upload a .deb file to a package",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := newCloudClient()
			if err != nil {
				return err
			}
			return uploadDebToPackage(c, args[0], filePath)
		},
	}
	cmd.Flags().StringVar(&filePath, "file", "", "path to .deb file (required)")
	cmd.MarkFlagRequired("file")
	return cmd
}

// uploadDebToPackage uploads a .deb file to an existing package. Shared by
// pkgUploadDebCmd, pkgCreateCmd --deb, cloud run --deb, and cloud bench --*-deb.
func uploadDebToPackage(c *cloudHTTPClient, packageID, debPath string) error {
	f, err := os.Open(debPath)
	if err != nil {
		return fmt.Errorf("open deb file: %w", err)
	}
	defer f.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(debPath))
	if err != nil {
		return err
	}
	if _, err := io.Copy(part, f); err != nil {
		return err
	}
	writer.Close()

	req, err := http.NewRequest("POST", c.url("/api/v1/packages/"+packageID+"/deb"), body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.do(req)
	if err != nil {
		return fmt.Errorf("upload deb: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != 200 {
		return fmt.Errorf("deb upload failed %d: %s", resp.StatusCode, string(respBody))
	}

	var result struct {
		Filename string `json:"filename"`
		Size     string `json:"size"`
	}
	json.Unmarshal(respBody, &result)
	fmt.Printf("Uploaded deb: %s (%s bytes)\n", result.Filename, result.Size)
	return nil
}

// createPackageWithDeb creates a package and optionally uploads a .deb to it.
// Returns the package ID.
func createPackageWithDeb(c *cloudHTTPClient, name, dbKind, dbVersion, debPath string) (string, error) {
	body := map[string]any{
		"name":    name,
		"db_kind": dbKind,
	}
	if dbVersion != "" {
		body["db_version"] = dbVersion
	}

	raw, _ := json.Marshal(body)
	data, status, err := c.doJSON("POST", "/api/v1/packages", bytes.NewReader(raw))
	if err != nil {
		return "", fmt.Errorf("create package: %w", err)
	}
	if status != 201 {
		return "", fmt.Errorf("create package failed %d: %s", status, string(data))
	}

	var result struct {
		ID string `json:"id"`
	}
	json.Unmarshal(data, &result)
	fmt.Printf("Created package: %s\n", result.ID)

	if debPath != "" {
		if err := uploadDebToPackage(c, result.ID, debPath); err != nil {
			return "", err
		}
	}

	return result.ID, nil
}
