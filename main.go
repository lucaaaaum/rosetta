package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

type Project struct {
	PropertyGroups []PropertyGroup `xml:"PropertyGroup"`
	ItemGroups     []ItemGroup     `xml:"ItemGroup"`
}

type PropertyGroup struct {
	ManagePackageVersionsCentrally bool
}

type ItemGroup struct {
	PackageReferences []PackageReference `xml:"PackageReference"`
	PackageVersions   []PackageReference `xml:"PackageVersion"`
}

type PackageReference struct {
	Include string `xml:"Include,attr"`
	Version string `xml:"Version,attr"`
}

type Package struct {
	Name    string
	Version string
	Hash    string
	Source  string
}

type Assets struct {
	Libraries map[string]Library `json:"libraries"`
}

type Library struct {
}

var rootCmd = &cobra.Command{
	Use:   "rosetta",
	Short: "rosetta is a tool for generating the deps.nix file, required to build .NET projects with nix",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		rootDir := args[0]
		skipPackagesNotFound, _ := cmd.Flags().GetBool("skip-packages-not-found")

		if _, err := os.Stat(rootDir); os.IsNotExist(err) {
			return err
		}

		command := exec.Command("dotnet", "restore", rootDir)
		command.Stdout = os.Stdout
		command.Stderr = os.Stderr

		err := command.Run()

		if err != nil {
			return err
		}
		println("Restored .NET packages")

		assetJsonPaths := make([]string, 0)
		filepath.Walk(rootDir, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if !info.IsDir() && info.Name() == "project.assets.json" {
				assetJsonPaths = append(assetJsonPaths, path)
			}

			return nil
		})

		packages := make([]*Package, 0)

		for _, assetJsonPath := range assetJsonPaths {
			println("Processing asset file:", assetJsonPath)
			assetJsonFile, err := os.ReadFile(assetJsonPath)
			if err != nil {
				return err
			}

			var assets Assets
			err = json.Unmarshal(assetJsonFile, &assets)
			if err != nil {
				return err
			}

			for libKey := range assets.Libraries {
				parts := strings.SplitN(libKey, "/", 2)
				if len(parts) != 2 {
					continue
				}
				pkgName := parts[0]
				pkgVersion := parts[1]

				pkg := &Package{
					Name:    pkgName,
					Version: pkgVersion,
				}
				packages = append(packages, pkg)
			}
		}

		homeDir, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		nugetDir := path.Join(homeDir, ".nuget", "packages")
		for _, pkg := range packages {
			pkgNameLowerCase := strings.ToLower(pkg.Name)
			pkgVersionLowerCase := strings.ToLower(pkg.Version)
			nupkgPath := path.Join(nugetDir, pkgNameLowerCase, pkgVersionLowerCase, pkgNameLowerCase+"."+pkgVersionLowerCase+".nupkg")
			hashCommand := exec.Command("nix-hash", "--type", "sha256", "--sri", "--flat", nupkgPath)
			hashOutput, err := hashCommand.Output()
			if err != nil {
				println("Failed to resolve package:", pkg.Name, pkg.Version)
				if skipPackagesNotFound {
					continue
				}
				return err
			}
			pkg.Hash = strings.TrimSpace(string(hashOutput))
			println("Resolved package:", pkg.Name, pkg.Version, pkg.Hash)
		}

		depsNixFile, err := os.Create("deps.nix")
		if err != nil {
			return err
		}
		defer depsNixFile.Close()

		depsNixFile.WriteString("{ fetchNuGet }: [\n")

		for _, pkg := range packages {
			if pkg.Hash == "" {
				continue
			}
			depsNixFile.WriteString("  (fetchNuGet {\n")
			depsNixFile.WriteString("    pname = \"" + pkg.Name + "\";\n")
			depsNixFile.WriteString("    version = \"" + pkg.Version + "\";\n")
			depsNixFile.WriteString("    sha256 = \"" + pkg.Hash + "\";\n")
			depsNixFile.WriteString("  })\n")
		}

		depsNixFile.WriteString("]")

		return nil
	},
}

func main() {
	rootCmd.Flags().BoolP("skip-packages-not-found", "s", false, "Skip packages that cannot be found in the local NuGet cache")
	rootCmd.Execute()
}
