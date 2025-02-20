// Copyright 2021 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"io/ioutil"
	"os"
	"testing"

	"github.com/apigee/registry/rpc"
	"github.com/stretchr/testify/assert"
)

func setupFakeSpec() (path string, err error) {
	tempDir, err := ioutil.TempDir("", "")
	if err != nil {
		return "", err
	}

	f, err := ioutil.TempFile(tempDir, "*.proto")
	if err != nil {
		return "", err
	}
	return f.Name(), err
}

func TestApiLinterPluginLintSpec(t *testing.T) {
	specDirectory, err := setupFakeSpec()
	defer os.RemoveAll(specDirectory)
	assert.Equal(t, err, nil)

	lintSpecTests := []struct {
		linter           *apiLinterRunner
		runLinter        runLinter
		request          *rpc.LinterRequest
		expectedResponse *rpc.LinterResponse
		expectedError    error
	}{
		{
			&apiLinterRunner{},
			func(specPath string) ([]*rpc.LintProblem, error) {
				return []*rpc.LintProblem{
						{
							Message: "test",
							RuleId:  "test",
							Location: &rpc.LintLocation{
								StartPosition: &rpc.LintPosition{
									LineNumber:   1,
									ColumnNumber: 1,
								},
								EndPosition: &rpc.LintPosition{
									LineNumber:   3,
									ColumnNumber: 10,
								},
							},
						},
					},
					nil
			},
			&rpc.LinterRequest{
				SpecDirectory: specDirectory,
				RuleIds:       []string{"test"},
			},
			&rpc.LinterResponse{
				Lint: &rpc.Lint{
					Name: "registry-lint-api-linter",
					Files: []*rpc.LintFile{
						{
							FilePath: specDirectory,
							Problems: []*rpc.LintProblem{
								{
									Message: "test",
									RuleId:  "test",
									Location: &rpc.LintLocation{
										StartPosition: &rpc.LintPosition{
											LineNumber:   1,
											ColumnNumber: 1,
										},
										EndPosition: &rpc.LintPosition{
											LineNumber:   3,
											ColumnNumber: 10,
										},
									},
								},
							},
						},
					},
				},
			},
			nil,
		},
		{
			&apiLinterRunner{},
			func(specPath string) ([]*rpc.LintProblem, error) {
				return []*rpc.LintProblem{}, errors.New("test")
			},
			&rpc.LinterRequest{
				SpecDirectory: specDirectory,
			},
			nil,
			errors.New("test"),
		},
	}

	for _, tt := range lintSpecTests {
		response, err := tt.linter.RunImpl(tt.request, tt.runLinter)
		assert.Equal(t, tt.expectedError, err)
		assert.EqualValues(t, tt.expectedResponse, response)
	}
}
