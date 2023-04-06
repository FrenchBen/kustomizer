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

package registry

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/google/go-containerregistry/pkg/crane"
	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/empty"

	"github.com/containers/image/v5/transports/alltransports"
)

func Build(ctx context.Context, imageNames []string, data []byte) (string, error) {
	srcRef, err := alltransports.ParseImageName(imageNames[0])
	if err != nil {
		return fmt.Errorf("Invalid source name %s: %v", imageNames[0], err)
	}
	destRef, err := alltransports.ParseImageName(imageNames[1])
	if err != nil {
		return fmt.Errorf("Invalid destination name %s: %v", imageNames[1], err)
	}
	ref, err := name.ParseReference(url)
	if err != nil {
		return "", fmt.Errorf("parsing reference failed: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "oci")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	tarFile := filepath.Join(tmpDir, "all.tar")
	dataFile := "all.yaml"
	if err := tarContent(tarFile, dataFile, data); err != nil {
		return "", err
	}

	img, err := crane.Append(empty.Image, tarFile)
	if err != nil {
		return "", fmt.Errorf("appending content failed: %w", err)
	}

	return ref.Context().Digest(digest.String()).String(), nil
}
