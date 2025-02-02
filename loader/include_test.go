/*
   Copyright 2020 The Compose Specification Authors.

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

package loader

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/compose-spec/compose-go/v2/types"
	"gotest.tools/v3/assert"
)

func TestLoadWithMultipleInclude(t *testing.T) {
	// include same service twice should not trigger an error
	p, err := Load(buildConfigDetails(`
name: 'test-multi-include'

include:
  - path: ./testdata/subdir/compose-test-extends-imported.yaml
    env_file: ./testdata/subdir/extra.env
  - path: ./testdata/compose-include.yaml

services:
  foo:
    image: busybox
    depends_on:
      - imported
`, map[string]string{"SOURCE": "override"}), func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.NilError(t, err)
	imported, err := p.GetService("imported")
	assert.NilError(t, err)
	assert.Equal(t, imported.ContainerName, "override")

	// include 2 different services with same name should trigger an error
	_, err = Load(buildConfigDetails(`
name: 'test-multi-include'

include:
  - path: ./testdata/subdir/compose-test-extends-imported.yaml
    env_file: ./testdata/subdir/extra.env
  - path: ./testdata/compose-include.yaml
    env_file: ./testdata/subdir/extra.env


services:
  bar:
    image: busybox
`, map[string]string{"SOURCE": "override"}), func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
	})
	assert.ErrorContains(t, err, "services.bar conflicts with imported resource", err)
}

func TestIncludeRelative(t *testing.T) {
	wd, err := filepath.Abs(filepath.Join("testdata", "include"))
	assert.NilError(t, err)
	p, err := LoadWithContext(context.Background(), types.ConfigDetails{
		ConfigFiles: []types.ConfigFile{
			{
				Filename: filepath.Join("testdata", "include", "compose.yaml"),
			},
		},
		WorkingDir: wd,
	}, func(options *Options) {
		options.projectName = "test-include-relative"
		options.ResolvePaths = false
	})
	assert.NilError(t, err)
	included := p.Services["included"]
	assert.Equal(t, included.Build.Context, ".")
	assert.Equal(t, included.Volumes[0].Source, ".")
}

func TestLoadWithIncludeEnv(t *testing.T) {
	fileName := "compose.yml"
	tmpdir := t.TempDir()
	// file in root
	yaml := `
include:
  - path:
    - ./module/compose.yml
    env_file:
      - ./custom.env
services:
  a:
    image: alpine
    environment:
      - VAR_NAME`
	createFile(t, tmpdir, `VAR_NAME=value`, "custom.env")
	path := createFile(t, tmpdir, yaml, fileName)
	// file in /module
	yaml = `
services:
  b:
    image: alpine
    environment:
      - VAR_NAME
  c:
    image: alpine
    environment:
      - VAR_NAME`
	createFileSubDir(t, tmpdir, "module", yaml, fileName)

	p, err := Load(types.ConfigDetails{
		WorkingDir: tmpdir,
		ConfigFiles: []types.ConfigFile{{
			Filename: path,
		}},
		Environment: nil,
	}, func(options *Options) {
		options.SkipNormalization = true
		options.ResolvePaths = true
		options.SetProjectName("project", true)
	})
	assert.NilError(t, err)
	a := p.Services["a"]
	// make sure VAR_NAME is only accessible in include context
	assert.Check(t, a.Environment["VAR_NAME"] == nil, "VAR_NAME should not be defined in environment")
	b := p.Services["b"]
	assert.Check(t, b.Environment["VAR_NAME"] != nil, "VAR_NAME is not defined in environment")
	assert.Equal(t, *b.Environment["VAR_NAME"], "value")
	c := p.Services["c"]
	assert.Check(t, c.Environment["VAR_NAME"] != nil, "VAR_NAME is not defined in environment")
	assert.Equal(t, *c.Environment["VAR_NAME"], "value")

}

func createFile(t *testing.T, rootDir, content, fileName string) string {
	path := filepath.Join(rootDir, fileName)
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}

func createFileSubDir(t *testing.T, rootDir, subDir, content, fileName string) string {
	subDirPath := filepath.Join(rootDir, subDir)
	assert.NilError(t, os.Mkdir(subDirPath, 0o700))
	path := filepath.Join(subDirPath, fileName)
	assert.NilError(t, os.WriteFile(path, []byte(content), 0o600))
	return path
}
