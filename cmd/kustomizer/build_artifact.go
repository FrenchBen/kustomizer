/*
Copyright 2021 Stefan Prodan

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/fluxcd/pkg/ssa"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/spf13/cobra"
	"github.com/stefanprodan/kustomizer/pkg/registry"
)

var buildArtifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Build generates an inventory and writes the resulting artifact to the oci target.",
	Example: `  kustomizer build artifact docker.io/user/repo --kustomize <overlay path> [--file <dir path>|<file path>] [--format oci-archive|oci-dir|docker-dir|docker-archive] [--output <filename or dir>]

  # Build from Docker Hub registry into a local OCI archive
  kustomizer build artifact docker.io/user/repo:$(git rev-parse --short HEAD) \
	--file ./deploy/manifests

  # Build to a local OCI archive
  kustomizer build artifact docker.io/user/repo:$(git rev-parse --short HEAD) --format oci-archive --output repo-archive-$(git tag --points-at HEAD).tar \
	--kustomize="./deploy/production" \
	
`,
	RunE: runBuildArtifactCmd,
}

type buildArtifactFlags struct {
	filename  []string
	kustomize string
	patch     []string
	output    string
	format    string
}

var buildArtifactArgs buildArtifactFlags

func init() {
	buildArtifactCmd.Flags().StringSliceVarP(&buildArtifactArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	buildArtifactCmd.Flags().StringVarP(&buildArtifactArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	buildArtifactCmd.Flags().StringVarP(&buildArtifactArgs.format, "format", "", "oci-archive",
		"Save image to oci-archive, docker-archive (default 'oci-archive')")
	buildArtifactCmd.Flags().StringVarP(&buildArtifactArgs.output, "output", "o", "",
		" If specified, write output to this path. (default: transform image name to user-repo.tar)")
	buildArtifactCmd.Flags().StringSliceVarP(&buildArtifactArgs.patch, "patch", "p", nil,
		"Path to a kustomization file that contains a list of patches.")

	buildCmd.AddCommand(buildArtifactCmd)
}

func runBuildArtifactCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an artifact name e.g. 'docker.io/user/repo:tag'")
	}

	if buildArtifactArgs.kustomize == "" && len(buildArtifactArgs.filename) == 0 {
		return fmt.Errorf("-f or -k is required")
	}

	if !validateFormat(buildArtifactArgs.format) {
		return fmt.Errorf("valid formats are: oci-archive, docker-archive")
	}

	url := args[0]
	imgRef, err := name.NewTag(url)
	if err != nil {
		return fmt.Errorf("invalid image name %s: %v", url, err)
	}

	outputFile := buildArtifactArgs.output
	if outputFile == "" {
		repo := imgRef.Repository.RepositoryStr()
		if imgRef.Repository.Registry.Name() == "docker:" && repo[0:1] == "/" {
			repo = repo[1:]
		}
		outputFile = fmt.Sprintf("%s.img", strings.Replace(repo, "/", "-", -1))
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	logger.Println("building manifest...")
	objects, _, err := buildManifests(ctx, buildArtifactArgs.kustomize, buildArtifactArgs.filename, nil, buildArtifactArgs.patch, nil)
	if err != nil {
		return err
	}

	sort.Sort(ssa.SortableUnstructureds(objects))

	for _, object := range objects {
		rootCmd.Println(ssa.FmtUnstructured(object))
	}

	yml, err := ssa.ObjectsToYAML(objects)
	if err != nil {
		return err
	}

	if err := registry.Build(ctx, url, buildArtifactArgs.format, outputFile, []byte(yml)); err != nil {
		return fmt.Errorf("building archive failed: %w", err)
	}

	logger.Println("bluilt image archive at ", outputFile)
	return nil
}

func validateFormat(format string) bool {
	validFormats := []string{"oci-archive", "docker-archive"}
	for _, v := range validFormats {
		if v == format {
			return true
		}
	}

	return false
}
