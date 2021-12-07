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
	"os"
	"sort"

	"github.com/stefanprodan/kustomizer/pkg/inventory"

	"github.com/fluxcd/pkg/ssa"
	"github.com/spf13/cobra"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "Delete the Kubernetes objects in the inventory including the inventory configmap.",
	RunE:  deleteCmdRun,
}

type deleteFlags struct {
	inventoryName      string
	inventoryNamespace string
	wait               bool
}

var deleteArgs deleteFlags

func init() {
	deleteCmd.Flags().StringVarP(&deleteArgs.inventoryName, "inventory-name", "i", "", "The name of the inventory configmap.")
	deleteCmd.Flags().StringVar(&deleteArgs.inventoryNamespace, "inventory-namespace", "default", "The namespace of the inventory configmap.")
	deleteCmd.Flags().BoolVar(&deleteArgs.wait, "wait", true, "Wait for the deleted Kubernetes objects to be terminated.")

	rootCmd.AddCommand(deleteCmd)
}

func deleteCmdRun(cmd *cobra.Command, args []string) error {
	if deleteArgs.inventoryName == "" {
		return fmt.Errorf("--inventory-name is required")
	}
	if deleteArgs.inventoryNamespace == "" {
		return fmt.Errorf("--inventory-namespace is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), rootArgs.timeout)
	defer cancel()

	logger.Println("retrieving inventory...")

	kubeClient, err := newKubeClient(kubeconfigArgs)
	if err != nil {
		return fmt.Errorf("client init failed: %w", err)
	}

	statusPoller, err := newKubeStatusPoller(kubeconfigArgs)
	if err != nil {
		return fmt.Errorf("status poller init failed: %w", err)
	}

	resMgr := ssa.NewResourceManager(kubeClient, statusPoller, inventoryOwner)

	invStorage := &inventory.InventoryStorage{
		Manager: resMgr,
		Owner:   inventoryOwner,
	}

	inv := inventory.NewInventory(deleteArgs.inventoryName, deleteArgs.inventoryNamespace)
	if err := invStorage.GetInventory(ctx, inv); err != nil {
		return err
	}

	objects, err := inv.ListObjects()
	if err != nil {
		return err
	}

	logger.Println(fmt.Sprintf("deleting %v manifest(s)...", len(objects)))
	hasErrors := false
	sort.Sort(sort.Reverse(ssa.SortableUnstructureds(objects)))
	for _, object := range objects {
		change, err := resMgr.Delete(ctx, object, ssa.DefaultDeleteOptions())
		if err != nil {
			logger.Println(`✗`, err)
			hasErrors = true
			continue
		}
		logger.Println(change.String())
	}

	if hasErrors {
		os.Exit(1)
	}

	if err := invStorage.DeleteInventory(ctx, inv); err != nil {
		return err
	}

	logger.Println(fmt.Sprintf("ConfigMap/%s/%s deleted", deleteArgs.inventoryNamespace, deleteArgs.inventoryName))

	if deleteArgs.wait {
		waitOpts := ssa.DefaultWaitOptions()
		waitOpts.Timeout = rootArgs.timeout
		logger.Println("waiting for resources to be terminated...")
		err = resMgr.WaitForTermination(objects, waitOpts)
		if err != nil {
			return err
		}
		logger.Println("all resources have been deleted")
	}

	return nil
}
