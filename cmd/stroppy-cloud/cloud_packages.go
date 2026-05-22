package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"text/tabwriter"

	"github.com/spf13/cobra"

	uipb "github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/api/ui"
	"github.com/stroppy-io/stroppy-cloud/internal/proto/cloud/v1/models"
)

func cloudPackagesCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "packages", Short: "Manage database packages"}
	cmd.AddCommand(
		pkgListCmd(),
		pkgGetCmd(),
		pkgCreateCmd(),
		pkgUpdateCmd(),
		pkgDeleteCmd(),
		pkgCloneCmd(),
		pkgUploadCmd(),
	)
	return cmd
}

func pkgListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List database packages (PackageService.ListPackages)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()

			tenantID, err := c.tenantID()
			if err != nil {
				return err
			}
			ctx, cancel := callCtx()
			defer cancel()
			list, err := uipb.NewPackageServiceClient(c.conn).ListPackages(ctx, &uipb.ListPackagesRequest{
				TenantId: tenantID,
			})
			if err != nil {
				return fmt.Errorf("list packages: %w", err)
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 4, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tNAME\tDB\tVER\tBUILTIN\tDEB")
			for _, p := range list.GetPackages() {
				builtin := "no"
				if p.GetIsBuiltin() {
					builtin = "yes"
				}
				deb := "-"
				if p.GetDebObjectUri() != "" {
					deb = p.GetDebObjectUri()
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					p.GetEntity().GetId().GetValue(), p.GetName(), p.GetDbKind(), p.GetDbVersion(), builtin, deb)
			}
			return w.Flush()
		},
	}
}

func pkgGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "get <package-id>",
		Short: "Show package details (degraded: no per-package RPC on the server)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("packages get is not supported: the ui PackageService exposes only ListPackages, RequestPackageUpload and DeletePackage — use `cloud packages list` and filter by id %q", args[0])
		},
	}
}

func pkgCreateCmd() *cobra.Command {
	var (
		name      string
		dbKind    string
		dbVersion string
		debFile   string
	)
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a package via presigned upload (PackageService.RequestPackageUpload + PUT)",
		Long: `Create a package catalog record and upload its .deb.

The server has no standalone create RPC: package creation is fused with the
upload flow (RequestPackageUpload returns the created record plus a presigned
PUT URL). --deb is therefore required.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if debFile == "" {
				return fmt.Errorf("--deb is required: the server creates a package only via the presigned upload flow")
			}
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()
			id, err := uploadPackage(c, name, dbKind, dbVersion, debFile)
			if err != nil {
				return err
			}
			fmt.Printf("Created package: %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "package name (required)")
	cmd.Flags().StringVar(&dbKind, "db-kind", "", "database kind (required)")
	cmd.Flags().StringVar(&dbVersion, "db-version", "", "database version")
	cmd.Flags().StringVar(&debFile, "deb", "", "path to .deb file to upload (required)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("db-kind")
	return cmd
}

func pkgUpdateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "update <package-id>",
		Short: "Update a package (degraded: no update RPC on the server)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("packages update is not supported by the current ui PackageService (no UpdatePackage RPC); re-upload with `cloud packages upload` to replace the artifact")
		},
	}
}

func pkgDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <package-id>",
		Short: "Delete a package (PackageService.DeletePackage)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()
			tenantID, err := c.tenantID()
			if err != nil {
				return err
			}
			ctx, cancel := callCtx()
			defer cancel()
			_, err = uipb.NewPackageServiceClient(c.conn).DeletePackage(ctx, &uipb.DeletePackageRequest{
				TenantId: tenantID,
				Id:       &models.Ulid{Value: args[0]},
			})
			if err != nil {
				return fmt.Errorf("delete package: %w", err)
			}
			fmt.Println("Package deleted.")
			return nil
		},
	}
}

func pkgCloneCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "clone <package-id>",
		Short: "Clone a package (degraded: no clone RPC on the server)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("packages clone is not supported by the current ui PackageService (no ClonePackage RPC)")
		},
	}
}

func pkgUploadCmd() *cobra.Command {
	var (
		name      string
		dbKind    string
		dbVersion string
		debFile   string
	)
	cmd := &cobra.Command{
		Use:   "upload",
		Short: "Upload a .deb via presigned PUT (PackageService.RequestPackageUpload)",
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := dialClient()
			if err != nil {
				return err
			}
			defer c.close()
			id, err := uploadPackage(c, name, dbKind, dbVersion, debFile)
			if err != nil {
				return err
			}
			fmt.Printf("Uploaded package: %s\n", id)
			return nil
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "package name (required)")
	cmd.Flags().StringVar(&dbKind, "db-kind", "", "database kind (required)")
	cmd.Flags().StringVar(&dbVersion, "db-version", "", "database version")
	cmd.Flags().StringVar(&debFile, "deb", "", "path to .deb file (required)")
	_ = cmd.MarkFlagRequired("name")
	_ = cmd.MarkFlagRequired("db-kind")
	_ = cmd.MarkFlagRequired("deb")
	return cmd
}

// uploadPackage runs the presigned-upload flow: RequestPackageUpload returns the
// created record + a presigned PUT URL, then the .deb is PUT to that URL.
// Returns the new package id.
func uploadPackage(c *cloudClient, name, dbKind, dbVersion, debPath string) (string, error) {
	tenantID, err := c.tenantID()
	if err != nil {
		return "", err
	}
	f, err := os.Open(debPath)
	if err != nil {
		return "", fmt.Errorf("open deb file: %w", err)
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat deb file: %w", err)
	}

	ctx, cancel := callCtx()
	defer cancel()
	resp, err := uipb.NewPackageServiceClient(c.conn).RequestPackageUpload(ctx, &uipb.RequestPackageUploadRequest{
		TenantId:    tenantID,
		Name:        name,
		DbKind:      dbKind,
		DbVersion:   dbVersion,
		DebFilename: filepath.Base(debPath),
	})
	if err != nil {
		return "", fmt.Errorf("request package upload: %w", err)
	}

	uploadURL := resp.GetUploadUrl()
	if uploadURL == "" {
		return "", fmt.Errorf("server returned no presigned upload URL")
	}

	req, err := http.NewRequest(http.MethodPut, uploadURL, f)
	if err != nil {
		return "", fmt.Errorf("build PUT request: %w", err)
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", "application/octet-stream")

	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload deb (PUT): %w", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode/100 != 2 {
		return "", fmt.Errorf("deb upload failed: HTTP %d", putResp.StatusCode)
	}

	fmt.Printf("Uploaded %s (%d bytes)\n", filepath.Base(debPath), info.Size())
	return resp.GetPackage().GetEntity().GetId().GetValue(), nil
}
