// Copyright 2020 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package core

import (
	"context"
	"fmt"
	"strings"

	"github.com/apigee/registry/rpc"
	"google.golang.org/protobuf/proto"

	metrics "github.com/google/gnostic/metrics"
)

func ExportVersionHistoryToSheet(ctx context.Context, name string, artifact *rpc.Artifact) (string, error) {
	sheetsClient, err := NewSheetsClient(ctx, "")
	if err != nil {
		return "", err
	}
	versionHistory, err := getVersionHistory(artifact)
	if err != nil {
		return "", err
	}
	sheetNames := []string{"Summary"}
	for _, version := range versionHistory.Versions {
		versionName := nameForVersion(version.Name)
		sheetNames = append(sheetNames, versionName+"-new")
		sheetNames = append(sheetNames, versionName+"-deleted")
	}
	sheet, err := sheetsClient.CreateSheet(name, sheetNames)
	if err != nil {
		return "", err
	}
	rows := make([][]interface{}, 0)
	rows = append(rows, rowForVersionSummary(nil))
	for _, version := range versionHistory.Versions {
		rows = append(rows, rowForVersionSummary(version))
	}
	_, err = sheetsClient.Update(ctx, "Summary", rows)
	if err != nil {
		return "", err
	}
	for _, version := range versionHistory.Versions {
		versionName := nameForVersion(version.Name)
		rows := rowsForVocabulary(version.NewTerms)
		_, err = sheetsClient.Update(ctx, versionName+"-new", rows)
		if err != nil {
			return "", err
		}
		rows = rowsForVocabulary(version.DeletedTerms)
		_, err = sheetsClient.Update(ctx, versionName+"-deleted", rows)
		if err != nil {
			return "", err
		}
	}
	return sheet.SpreadsheetUrl, nil
}

func nameForVersion(version string) string {
	parts := strings.Split(version, "/")
	return parts[5]
}

func getVersionHistory(artifact *rpc.Artifact) (*metrics.VersionHistory, error) {
	messageType, err := MessageTypeForMimeType(artifact.GetMimeType())
	if err == nil && messageType == "gnostic.metrics.VersionHistory" {
		value := &metrics.VersionHistory{}
		err := proto.Unmarshal(artifact.GetContents(), value)
		if err != nil {
			return nil, err
		} else {
			return value, nil
		}
	} else {
		return nil, fmt.Errorf("not a version history: %s", artifact.Name)
	}
}

func rowForVersionSummary(v *metrics.Version) []interface{} {
	if v == nil {
		return []interface{}{
			"version",
			"new terms",
			"deleted terms",
		}
	}
	version := v.Name
	return []interface{}{nameForVersion(version), v.NewTermCount, v.DeletedTermCount}
}

func rowsForVocabulary(vocabulary *metrics.Vocabulary) [][]interface{} {
	rows := make([][]interface{}, 0)
	rows = append(rows, rowForLabeledWordCount("", nil))
	for _, wc := range vocabulary.Schemas {
		rows = append(rows, rowForLabeledWordCount("schema", wc))
	}
	for _, wc := range vocabulary.Properties {
		rows = append(rows, rowForLabeledWordCount("artifact", wc))
	}
	for _, wc := range vocabulary.Operations {
		rows = append(rows, rowForLabeledWordCount("operation", wc))
	}
	for _, wc := range vocabulary.Parameters {
		rows = append(rows, rowForLabeledWordCount("parameter", wc))
	}
	return rows
}
