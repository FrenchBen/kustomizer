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
	"fmt"
	"path/filepath"
	"testing"

	. "github.com/onsi/gomega"
)

func TestBuildArchive(t *testing.T) {
	g := NewWithT(t)
	id := randStringRunes(5)
	tag := "v1.0.0"
	// create tmp location for asset output
	testDir := filepath.Join(tmpDir, id)
	outputImg := fmt.Sprintf("%s/kustomizer-%s.img", testDir, id)

	// TODO: repair bug in upstream package
	// BUG: current go-containerregistry parsing doesn't parse local instances ?
	// artifact := fmt.Sprintf("oci:%s/kustomizer/%s:%s", registryHost, id, tag)
	artifact := fmt.Sprintf("oci://ghcr.io/kustomizer/%s:%s", id, tag)
	artifactImg, err := buildOutput(artifact)
	g.Expect(err).NotTo(HaveOccurred())
	artifactImg = fmt.Sprintf("%s/%s", testDir, artifactImg)

	dir, err := makeTestDir(id, testManifests(id, id, false))
	g.Expect(err).NotTo(HaveOccurred())

	t.Run("build oci-archive image", func(t *testing.T) {
		// we override the output location,
		// and confirm the artifact was built in our temp dir to not pollute the repo
		output, err := executeCommand(fmt.Sprintf(
			"build artifact %s -k %s --output %s",
			artifact,
			dir,
			artifactImg,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(artifactImg))
	})

	t.Run("build docker-archive image", func(t *testing.T) {
		// we override the output location,
		// and confirm the artifact was built in our temp dir to not pollute the repo
		output, err := executeCommand(fmt.Sprintf(
			"build artifact %s -k %s --format docker-archive --output %s",
			artifact,
			dir,
			artifactImg,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(artifactImg))
	})

	t.Run("build oci-archive with output target", func(t *testing.T) {
		output, err := executeCommand(fmt.Sprintf(
			"build artifact %s -k %s -o %s",
			artifact,
			dir,
			outputImg,
		))

		g.Expect(err).NotTo(HaveOccurred())
		t.Logf("\n%s", output)
		g.Expect(output).To(MatchRegexp(outputImg))
	})
}
