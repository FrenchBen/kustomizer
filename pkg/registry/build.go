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
	"github.com/google/go-containerregistry/pkg/v1/empty"
	"github.com/google/go-containerregistry/pkg/v1/mutate"
	gcr "github.com/google/go-containerregistry/pkg/v1/types"

	"github.com/containers/image/v5/types"
)

func Build(ctx context.Context, image types.ImageReference, format, output string, data []byte) error {

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
	case "docker-dir", "docker-archive":
		base = mutate.MediaType(base, gcr.DockerManifestSchema2)
		base = mutate.ConfigMediaType(base, gcr.DockerConfigJSON)

	default:
		base = mutate.MediaType(base, gcr.OCIManifestSchema1)
		base = mutate.ConfigMediaType(base, gcr.OCIConfigJSON)
	}

	srcImg, err := crane.Append(base, tarFile)
	if err != nil {
		return fmt.Errorf("appending content failed: %w", err)
	}

	// -------------------------- new logic
	// options := &libimage.SaveOptions{}

	// Name the image as tmp
	// tag, err := name.NewTag(image.DockerReference().Name(), name.StrictValidation)
	// if err != nil {
	// 	return "", fmt.Errorf("creating new tmp tag failed: %w", err)
	// }

	// Use the name from destRef to create a file
	o, err := os.Create(output)
	if err != nil {
		return fmt.Errorf("creating %q to write image tarball failed %v", output, err)
	}
	defer o.Close()
	return crane.Export(srcImg, o)

	// if err := tarball.Write(tag, srcImg, o); err != nil {
	// 	return "", fmt.Errorf("Unexpected error writing tarball: %v", err)
	// }

	/*
		runtime := new(libimage.Runtime)
		storeOpts, err := storage.DefaultStoreOptions(true, rootless.GetRootlessUID())
		if err != nil {
			return nil, err
		}
		runtime.storageConfig = storeOpts












		switch format {
		case "oci-archive":
			destRef, err = ociArchiveTransport.NewReference(output, image.DockerReference().Name())

		case "oci-dir":
			destRef, err = ociTransport.NewReference(output, image.DockerReference().Name())
			options.ManifestMIMEType = ociv1.MediaTypeImageManifest

		case "docker-dir":
			destRef, err = dirTransport.NewReference(output)
			options.ManifestMIMEType = manifest.DockerV2Schema2MediaType

		case "docker-archive":
			destRef, err = dockerArchiveTransport.NewReference(output, image.DockerReference().Name())
			options.ManifestMIMEType = manifest.DockerV2Schema2MediaType

		default:
			return "", fmt.Errorf("unsupported format %q for saving images", format)
		}

		/*
				// Create a store for the runtime
				store, err := storage.GetStore(r.storageConfig)
				if err != nil {
					return err
				}

				runtime := new(libimage.Runtime)

				r.store = store
				is.Transport.SetStore(store)

				// Set up a storage service for creating container root filesystems from
				// images
				r.storageService = getStorageService(r.store)

				runtimeOptions := &libimage.RuntimeOptions{
					SystemContext: r.imageContext,
				}
				libimageRuntime, err := libimage.RuntimeFromStore(store, runtimeOptions)
				if err != nil {
					return err
				}
				r.libimageRuntime = libimageRuntime
				// Run the libimage events routine.
				r.libimageEvents()


			// Save the two images into a multi-image archive.  This way, we can
			// reload the images for each test.
			saveOptions := &libimage.SaveOptions{}
			saveOptions.Writer = os.Stdout
			imageCache, err := os.CreateTemp("", "saveimagecache")
			require.NoError(t, err)
			imageCache.Close()
			defer os.Remove(imageCache.Name())
			err = runtime.Save(ctx, []string{"alpine", "busybox"}, "docker-archive", imageCache.Name(), saveOptions)
	*/

	/*
		Need to override OS to be linux: `opts.overrideOS`
		Create src Ref - the Crane Image - parseImage is in OCI format oci://registry/image/app:tag
		Create dest Ref - the archive based on cli parameter oci-archive:some-archive-name.tar
		Create the context for the work ( OSChoice / overrideOS is set here)

		Do the work to copy....

	*/

	// srcRef, err := alltransports.ParseImageName(srcImg)
	// if err != nil {
	// 	return "", fmt.Errorf("Invalid source name %s: %v", srcImg, err)
	// }

	// sharedOpts := skopeo.ImageOptions{
	// 	skopeo.DockerImageOptions: skopeo.DockerImageOptions{
	// 		global: &skopeo.GlobalOptions{},
	// 		shared: &skopeo.SharedImageOptions{},
	// 	},
	// }
	// srcOpts := skopeo.ImageFlags(sharedOpts)
	// destOpts := skopeo.ImageDestFlags(sharedOpts)
	// opts := skopeo.CopyOptions{
	// 	global:    skopeo.Global,
	// 	srcImage:  srcOpts,
	// 	destImage: destOpts,
	// 	retryOpts: retry.Options{},
	// }

	// sourceCtx, err := opts.srcImage.newSystemContext()
	// if err != nil {
	// 	return err
	// }
	// destinationCtx, err := opts.destImage.newSystemContext()
	// if err != nil {
	// 	return err
	// }

	// Look at common image save:
	// https://github.com/containers/common/blob/main/libimage/save.go

	//  --override-os
	// src: docker://docker.apple.com/base-images/ubi9-minimal/ubi-minimal-runtime:latest
	// dest: oci-archive:ubi9-runtime.tar

}
