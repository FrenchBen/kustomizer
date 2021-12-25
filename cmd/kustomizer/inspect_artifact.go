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
	"strings"

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stefanprodan/kustomizer/pkg/registry"
)

var inspectArtifactCmd = &cobra.Command{
	Use:   "artifact",
	Short: "Inspect downloads the specified OCI artifact and prints a report of its content.",
	Long: `The inspect command downloads the specified OCI artifact and prints the artifact metadata,
lists the Kubernetes objects and the container image references.
For private registries, the inspect command uses the credentials from '~/.docker/config.json'.`,
	Example: ` kustomizer inspect artifact <oci url>

  # Inspect an OCI artifact
  kustomizer inspect artifact oci://docker.io/user/repo:latest

  # List only the container images references
  kustomizer inspect artifact oci://docker.io/user/repo:v1.0 --container-images
`,
	RunE: runInspectArtifactCmd,
}

type inspectArtifactFlags struct {
	containerImages bool
}

var inspectArtifactArgs inspectArtifactFlags

func init() {
	inspectArtifactCmd.Flags().BoolVar(&inspectArtifactArgs.containerImages, "container-images", false,
		"List only the container images referenced in the Kubernetes manifests.")

	inspectCmd.AddCommand(inspectArtifactCmd)
}

func runInspectArtifactCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an OCI URL e.g. 'oci://docker.io/user/repo:tag'")
	}

	url, err := registry.ParseURL(args[0])
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	yml, meta, err := registry.Pull(ctx, url)
	if err != nil {
		return fmt.Errorf("pulling %s failed: %w", url, err)
	}

	objects, err := ssa.ReadObjects(strings.NewReader(yml))
	if err != nil {
		return err
	}

	if inspectArtifactArgs.containerImages {
		images := make(map[string]bool)
		for _, object := range objects {
			for _, image := range getContainerImages(object) {
				images[image] = true
			}
		}
		for image := range images {
			rootCmd.Println(image)
		}
		return nil
	}

	rootCmd.Println(fmt.Sprintf("Artifact: oci://%s", meta.Digest))
	rootCmd.Println("BuiltBy:", fmt.Sprintf("kustomizer/v%s", meta.Version))
	rootCmd.Println("CreatedAt:", meta.Created)
	rootCmd.Println("Checksum:", meta.Checksum)
	rootCmd.Println("Resources:")
	for _, object := range objects {
		rootCmd.Println("-", ssa.FmtUnstructured(object))
		images := getContainerImages(object)
		for _, image := range images {
			rootCmd.Println("  -", image)
		}
	}

	return nil
}

func getContainerImages(object *unstructured.Unstructured) []string {
	images := make(map[string]bool)
	var containers []interface{}

	// pod
	if cs, ok, _ := unstructured.NestedSlice(object.Object, "spec", "containers"); ok {
		containers = append(containers, cs...)
	}
	if cs, ok, _ := unstructured.NestedSlice(object.Object, "spec", "initContainers"); ok {
		containers = append(containers, cs...)
	}

	// job, deployment, statefulset, daemonset, knative service
	if cs, ok, _ := unstructured.NestedSlice(object.Object, "spec", "template", "spec", "containers"); ok {
		containers = append(containers, cs...)
	}
	if cs, ok, _ := unstructured.NestedSlice(object.Object, "spec", "template", "spec", "initContainers"); ok {
		containers = append(containers, cs...)
	}

	// cron job
	if cs, ok, _ := unstructured.NestedSlice(object.Object, "spec", "jobTemplate", "spec", "template", "spec", "containers"); ok {
		containers = append(containers, cs...)
	}
	if cs, ok, _ := unstructured.NestedSlice(object.Object, "spec", "jobTemplate", "spec", "template", "spec", "initContainers"); ok {
		containers = append(containers, cs...)
	}

	// tekton task
	if cs, ok, _ := unstructured.NestedSlice(object.Object, "spec", "steps"); ok {
		containers = append(containers, cs...)
	}

	for i := range containers {
		if c, ok := containers[i].(map[string]interface{}); ok {
			if image, ok, _ := unstructured.NestedString(c, "image"); ok {
				images[image] = true
			}
		}
	}

	var result []string
	for s, _ := range images {
		result = append(result, s)
	}

	return result
}
