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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"github.com/stefanprodan/kustomizer/pkg/inventory"
)

var applyInventoryCmd = &cobra.Command{
	Use:     "inventory",
	Aliases: []string{"inv"},
	Short:   "Apply builds the given inventory, then it validates and reconciles the Kubernetes objects using server-side apply.",
	Example: `  kustomizer apply inventory <name> [-a] [-p] [-f] -k --prune --wait --force --source --revision

  # Apply an inventory from remote OCI artifacts
  kustomizer apply inventory my-app -n apps -a oci://registry/org/repo:latest

  # Apply an inventory from remote OCI artifacts and local patches
  kustomizer apply inventory my-app -n apps -a oci://registry/org/repo:latest -p ./patches/safe-to-evict.yaml

  # Force apply a local kustomize overlay then wait for all resources to become ready
  kustomizer apply inventory my-app -n apps -k ./overlays/prod --prune --wait --force

  # Apply Kubernetes YAML manifests from a locally cloned Git repository
  kustomizer apply inventory my-app -n apps -f ./deploy/manifests --source="$(git ls-remote --get-url)" --revision="$(git describe --always)"
`,
	RunE: runApplyInventoryCmd,
}

type applyInventoryFlags struct {
	artifact        []string
	filename        []string
	kustomize       string
	patch           []string
	wait            bool
	force           bool
	prune           bool
	source          string
	revision        string
	createNamespace bool
}

var applyInventoryArgs applyInventoryFlags

func init() {
	applyInventoryCmd.Flags().StringSliceVarP(&applyInventoryArgs.filename, "filename", "f", nil,
		"Path to Kubernetes manifest(s). If a directory is specified, then all manifests in the directory tree will be processed recursively.")
	applyInventoryCmd.Flags().StringVarP(&applyInventoryArgs.kustomize, "kustomize", "k", "",
		"Path to a directory that contains a kustomization.yaml.")
	applyInventoryCmd.Flags().StringSliceVarP(&applyInventoryArgs.artifact, "artifact", "a", nil,
		"OCI artifact URL in the format 'oci://registry/org/repo:tag' e.g. 'oci://docker.io/stefanprodan/app-deploy:v1.0.0'.")
	applyInventoryCmd.Flags().StringSliceVarP(&applyInventoryArgs.patch, "patch", "p", nil,
		"Path to a kustomization file that contains a list of patches.")
	applyInventoryCmd.Flags().BoolVar(&applyInventoryArgs.wait, "wait", false, "Wait for the applied Kubernetes objects to become ready.")
	applyInventoryCmd.Flags().BoolVar(&applyInventoryArgs.force, "force", false, "Recreate objects that contain immutable fields changes.")
	applyInventoryCmd.Flags().BoolVar(&applyInventoryArgs.prune, "prune", false, "Delete stale objects from the cluster.")
	applyInventoryCmd.Flags().StringVar(&applyInventoryArgs.source, "source", "", "The URL to the source code.")
	applyInventoryCmd.Flags().StringVar(&applyInventoryArgs.revision, "revision", "", "The revision identifier.")
	applyInventoryCmd.Flags().BoolVar(&applyInventoryArgs.createNamespace, "create-namespace", false, "Create the inventory namespace if not present.")
	applyCmd.AddCommand(applyInventoryCmd)
}

func runApplyInventoryCmd(cmd *cobra.Command, args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("you must specify an inventory name")
	}
	name := args[0]

	if applyInventoryArgs.kustomize == "" && len(applyInventoryArgs.filename) == 0 && len(applyInventoryArgs.artifact) == 0 {
		return fmt.Errorf("-a, -f or -k is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	logger.Println("building inventory...")
	objects, digests, err := buildManifests(ctx, applyInventoryArgs.kustomize, applyInventoryArgs.filename, applyInventoryArgs.artifact, applyInventoryArgs.patch)
	if err != nil {
		return err
	}

	newInventory := inventory.NewInventory(name, *kubeconfigArgs.Namespace)
	newInventory.SetSource(applyInventoryArgs.source, applyInventoryArgs.revision, digests)
	if err := newInventory.AddObjects(objects); err != nil {
		return fmt.Errorf("creating inventory failed, error: %w", err)
	}
	logger.Println(fmt.Sprintf("applying %v manifest(s)...", len(objects)))

	for _, object := range objects {
		fixReplicasConflict(object, objects)
	}

	kubeClient, err := newKubeClient(kubeconfigArgs)
	if err != nil {
		return fmt.Errorf("client init failed: %w", err)
	}

	statusPoller, err := newKubeStatusPoller(kubeconfigArgs)
	if err != nil {
		return fmt.Errorf("status poller init failed: %w", err)
	}

	resMgr := ssa.NewResourceManager(kubeClient, statusPoller, inventoryOwner)
	resMgr.SetOwnerLabels(objects, name, *kubeconfigArgs.Namespace)

	invStorage := &inventory.Storage{
		Manager: resMgr,
		Owner:   inventoryOwner,
	}

	// contains only CRDs and Namespaces
	var stageOne []*unstructured.Unstructured

	// contains all objects except for CRDs and Namespaces
	var stageTwo []*unstructured.Unstructured

	for _, u := range objects {
		if ssa.IsClusterDefinition(u) {
			stageOne = append(stageOne, u)
		} else {
			stageTwo = append(stageTwo, u)
		}
	}

	applyOpts := ssa.DefaultApplyOptions()
	applyOpts.Force = applyInventoryArgs.force

	waitOpts := ssa.DefaultWaitOptions()
	waitOpts.Timeout = rootArgs.timeout

	if len(stageOne) > 0 {
		changeSet, err := resMgr.ApplyAll(ctx, stageOne, applyOpts)
		if err != nil {
			return err
		}
		for _, change := range changeSet.Entries {
			logger.Println(change.String())
		}

		if err := resMgr.Wait(stageOne, waitOpts); err != nil {
			return err
		}
	}

	sort.Sort(ssa.SortableUnstructureds(stageTwo))
	for _, object := range stageTwo {
		change, err := resMgr.Apply(ctx, object, applyOpts)
		if err != nil {
			return err
		}
		logger.Println(change.String())
	}

	staleObjects, err := invStorage.GetInventoryStaleObjects(ctx, newInventory)
	if err != nil {
		return fmt.Errorf("inventory query failed, error: %w", err)
	}

	err = invStorage.ApplyInventory(ctx, newInventory, applyInventoryArgs.createNamespace)
	if err != nil {
		return fmt.Errorf("inventory apply failed, error: %w", err)
	}

	if applyInventoryArgs.prune && len(staleObjects) > 0 {
		changeSet, err := resMgr.DeleteAll(ctx, staleObjects, ssa.DefaultDeleteOptions())
		if err != nil {
			return fmt.Errorf("prune failed, error: %w", err)
		}
		for _, change := range changeSet.Entries {
			logger.Println(change.String())
		}
	}

	if applyInventoryArgs.wait {
		logger.Println("waiting for resources to become ready...")

		err = resMgr.Wait(objects, waitOpts)
		if err != nil {
			return err
		}

		if applyInventoryArgs.prune && len(staleObjects) > 0 {

			err = resMgr.WaitForTermination(staleObjects, waitOpts)
			if err != nil {
				return fmt.Errorf("wating for termination failed, error: %w", err)
			}
		}

		logger.Println("all resources are ready")
	}

	return nil
}

// fixReplicasConflict removes the replicas field from the given workload if it's managed by an HPA
func fixReplicasConflict(object *unstructured.Unstructured, objects []*unstructured.Unstructured) {
	for _, hpa := range objects {
		if hpa.GetKind() == "HorizontalPodAutoscaler" && object.GetNamespace() == hpa.GetNamespace() {
			targetKind, found, err := unstructured.NestedFieldCopy(hpa.Object, "spec", "scaleTargetRef", "kind")
			if err == nil && found && fmt.Sprintf("%v", targetKind) == object.GetKind() {
				targetName, found, err := unstructured.NestedFieldCopy(hpa.Object, "spec", "scaleTargetRef", "name")
				if err == nil && found && fmt.Sprintf("%v", targetName) == object.GetName() {
					unstructured.RemoveNestedField(object.Object, "spec", "replicas")
				}
			}
		}
	}
}