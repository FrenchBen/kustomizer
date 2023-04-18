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
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
)

func Build(ctx context.Context, url, format, output string, data []byte) error {

	tmpDir, err := os.MkdirTemp("", "oci")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	tarFile := filepath.Join(tmpDir, "all.tar")
	dataFile := "all.yaml"

	if err := tarContent(tarFile, dataFile, data); err != nil {
		return err
	}
	base := empty.Image
	switch format {
	case "docker-archive":
		base = mutate.MediaType(base, types.DockerManifestSchema2)
		base = mutate.ConfigMediaType(base, types.DockerConfigJSON)

	default:
		base = mutate.MediaType(base, types.OCIManifestSchema1)
		base = mutate.ConfigMediaType(base, types.OCIConfigJSON)
	}

	srcImg, err := crane.Append(base, tarFile)
	if err != nil {
		return fmt.Errorf("appending content failed: %w", err)
	}

	// Use the name from destRef to create a file
	f, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("creating %q to write image tarball failed %v", output, err)
	}
	defer f.Close()

	tag, err := name.NewTag(url)
	if err != nil {
		panic(err)
	}
	return tarball.Write(tag, srcImg, f)
}
