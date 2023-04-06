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

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"
	"github.com/stefanprodan/kustomizer/pkg/registry"
)

var buildArtifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Build generates an inventory and writes the resulting artifact to oci-archive.",
	Example: `  kustomizer build artifact <oci url> -k <overlay path> [-f <dir path>|<file path>] <oci archive>

  # Build from Docker Hub registry into a local OCI archive
  kustomizer build artifact oci://docker.io/user/repo:$(git rev-parse --short HEAD) \
	-f ./deploy/manifests \
	oci-archive:repo-archive-$(git rev-parse --short HEAD).tar

  # Build from GitHub Container Registry into a local OCI archive
  kustomizer build artifact oci://ghcr.io/user/repo:$(git tag --points-at HEAD) \
	--kustomize="./deploy/production" \
	 oci-archive:repo-archive-$(git tag --points-at HEAD).tar
`,
	RunE: runBuildArtifactCmd,
}

type buildArtifactFlags struct {
	filename  []string
	kustomize string
	patch     []string
}

var buildArtifactArgs buildArtifactFlags

func init() {
	buildArtifactCmd.Flags().StringSliceVarP(&buildArtifactArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	buildArtifactCmd.Flags().StringVarP(&buildArtifactArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	buildArtifactCmd.Flags().StringSliceVarP(&buildArtifactArgs.patch, "patch", "p", nil,
		"Path to a kustomization file that contains a list of patches.")

	buildCmd.AddCommand(buildArtifactCmd)
}

func runBuildArtifactCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("you must specify an artifact name e.g. 'oci://docker.io/user/repo:tag' and archive name e.g. oci-archive:repo.tar")
	}

	if buildArtifactArgs.kustomize == "" && len(buildArtifactArgs.filename) == 0 {
		return fmt.Errorf("-f or -k is required")
	}

	imageNames := args

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	logger.Println("building manifests...")
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

	digest, err := registry.Build(ctx, imageNames, []byte(yml))
	if err != nil {
		return fmt.Errorf("building archive failed: %w", err)
	}

	logger.Println("bluild digest", digest)
	return nil
}
