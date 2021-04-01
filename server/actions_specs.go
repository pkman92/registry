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

package server

import (
	"context"
	"fmt"
	"sort"

	"github.com/apigee/registry/rpc"
	"github.com/apigee/registry/server/models"
	"github.com/apigee/registry/server/names"
	"github.com/apigee/registry/server/storage"
	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/api/iterator"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// CreateApiSpec handles the corresponding API request.
func (s *RegistryServer) CreateApiSpec(ctx context.Context, req *rpc.CreateApiSpecRequest) (*rpc.ApiSpec, error) {
	parent, err := names.ParseVersion(req.GetParent())
	if err != nil {
		return nil, invalidArgumentError(err)
	}

	name := parent.Spec(req.GetApiSpecId())
	if name.SpecID == "" {
		name.SpecID = names.GenerateID()
	}

	return s.createSpec(ctx, name, req.GetApiSpec())
}

func (s *RegistryServer) createSpec(ctx context.Context, name names.Spec, body *rpc.ApiSpec) (*rpc.ApiSpec, error) {
	client, err := s.getStorageClient(ctx)
	if err != nil {
		return nil, unavailableError(err)
	}
	defer s.releaseStorageClient(client)

	if _, err := getSpec(ctx, client, name); err == nil {
		return nil, alreadyExistsError(fmt.Errorf("API spec %q already exists", name))
	} else if !isNotFound(err) {
		return nil, err
	}

	if err := name.Validate(); err != nil {
		return nil, invalidArgumentError(err)
	}

	// Creation should only succeed when the parent exists.
	if _, err := getVersion(ctx, client, name.Version()); err != nil {
		return nil, err
	}

	spec, err := models.NewSpec(name, body)
	if err != nil {
		return nil, invalidArgumentError(err)
	}

	if err := saveSpecRevision(ctx, client, spec); err != nil {
		return nil, err
	}

	if err := saveSpecRevisionContents(ctx, client, spec, body.GetContents()); err != nil {
		return nil, err
	}

	message, err := spec.BasicMessage(name.String())
	if err != nil {
		return nil, internalError(err)
	}

	s.notify(rpc.Notification_CREATED, spec.RevisionName())
	return message, nil
}

// DeleteApiSpec handles the corresponding API request.
func (s *RegistryServer) DeleteApiSpec(ctx context.Context, req *rpc.DeleteApiSpecRequest) (*empty.Empty, error) {
	client, err := s.getStorageClient(ctx)
	if err != nil {
		return nil, unavailableError(err)
	}
	defer s.releaseStorageClient(client)

	name, err := names.ParseSpec(req.GetName())
	if err != nil {
		return nil, invalidArgumentError(err)
	}

	// Deletion should only succeed on API specs that currently exist.
	if _, err := getSpec(ctx, client, name); err != nil {
		return nil, err
	}

	if err := deleteSpec(ctx, client, name); err != nil {
		return nil, err
	}

	s.notify(rpc.Notification_DELETED, name.String())
	return &empty.Empty{}, nil
}

// GetApiSpec handles the corresponding API request.
func (s *RegistryServer) GetApiSpec(ctx context.Context, req *rpc.GetApiSpecRequest) (*rpc.ApiSpec, error) {
	if name, err := names.ParseSpec(req.GetName()); err == nil {
		return s.getApiSpec(ctx, name, req.GetView())
	} else if name, err := names.ParseSpecRevision(req.GetName()); err == nil {
		return s.getApiSpecRevision(ctx, name, req.GetView())
	}

	return nil, invalidArgumentError(fmt.Errorf("invalid resource name %q, must be an API spec or revision", req.GetName()))
}

func (s *RegistryServer) getApiSpec(ctx context.Context, name names.Spec, view rpc.View) (*rpc.ApiSpec, error) {
	client, err := s.getStorageClient(ctx)
	if err != nil {
		return nil, unavailableError(err)
	}
	defer s.releaseStorageClient(client)

	spec, err := getSpec(ctx, client, name)
	if err != nil {
		return nil, err
	}

	blob, err := getSpecRevisionContents(ctx, client, name.Revision(spec.RevisionID))
	if err != nil {
		return nil, err
	}

	var message *rpc.ApiSpec
	if view == rpc.View_FULL {
		message, err = spec.FullMessage(blob, name.String())
		if err != nil {
			return nil, internalError(err)
		}
	} else {
		message, err = spec.BasicMessage(name.String())
		if err != nil {
			return nil, internalError(err)
		}
	}

	return message, nil
}

func (s *RegistryServer) getApiSpecRevision(ctx context.Context, name names.SpecRevision, view rpc.View) (*rpc.ApiSpec, error) {
	client, err := s.getStorageClient(ctx)
	if err != nil {
		return nil, unavailableError(err)
	}
	defer s.releaseStorageClient(client)

	revision, err := getSpecRevision(ctx, client, name)
	if err != nil {
		return nil, err
	}

	blob, err := getSpecRevisionContents(ctx, client, name)
	if err != nil {
		return nil, err
	}

	var message *rpc.ApiSpec
	if view == rpc.View_FULL {
		message, err = revision.FullMessage(blob, name.String())
		if err != nil {
			return nil, internalError(err)
		}
	} else {
		message, err = revision.BasicMessage(name.String())
		if err != nil {
			return nil, internalError(err)
		}
	}

	return message, nil
}

// ListApiSpecs handles the corresponding API request.
func (s *RegistryServer) ListApiSpecs(ctx context.Context, req *rpc.ListApiSpecsRequest) (*rpc.ListApiSpecsResponse, error) {
	client, err := s.getStorageClient(ctx)
	if err != nil {
		return nil, unavailableError(err)
	}
	defer s.releaseStorageClient(client)

	if req.GetPageSize() < 0 {
		return nil, invalidArgumentError(fmt.Errorf("invalid page_size %d: must not be negative", req.GetPageSize()))
	}

	q := client.NewQuery(storage.SpecEntityName)
	q, err = q.ApplyCursor(req.GetPageToken())
	if err != nil {
		return nil, invalidArgumentError(err)
	}
	parent, err := names.ParseVersion(req.GetParent())
	if err != nil {
		return nil, invalidArgumentError(err)
	}
	if id := parent.ProjectID; id != "-" {
		q = q.Require("ProjectID", id)
	}
	if id := parent.ApiID; id != "-" {
		q = q.Require("ApiID", id)
	}
	if id := parent.VersionID; id != "-" {
		q = q.Require("VersionID", id)
	}

	if parent.ProjectID != "-" && parent.ApiID != "-" && parent.VersionID != "-" {
		if _, err := getVersion(ctx, client, parent); err != nil {
			return nil, err
		}
	} else if parent.ProjectID != "-" && parent.ApiID != "-" && parent.VersionID == "-" {
		if _, err := getApi(ctx, client, parent.Api()); err != nil {
			return nil, err
		}
	} else if parent.ProjectID != "-" && parent.ApiID == "-" && parent.VersionID == "-" {
		if _, err := getProject(ctx, client, parent.Project()); err != nil {
			return nil, err
		}
	}

	q = q.Require("Currency", models.IsCurrent)
	prg, err := createFilterOperator(req.GetFilter(),
		[]filterArg{
			{"name", filterArgTypeString},
			{"project_id", filterArgTypeString},
			{"api_id", filterArgTypeString},
			{"version_id", filterArgTypeString},
			{"spec_id", filterArgTypeString},
			{"filename", filterArgTypeString},
			{"description", filterArgTypeString},
			{"create_time", filterArgTypeTimestamp},
			{"revision_create_time", filterArgTypeTimestamp},
			{"revision_update_time", filterArgTypeTimestamp},
			{"mime_type", filterArgTypeString},
			{"size_bytes", filterArgTypeInt},
			{"source_uri", filterArgTypeString},
			{"labels", filterArgTypeStringMap},
		})
	if err != nil {
		return nil, internalError(err)
	}
	var specMessages []*rpc.ApiSpec
	var spec models.Spec
	it := client.Run(ctx, q)
	pageSize := boundPageSize(req.GetPageSize())
	for _, err := it.Next(&spec); err == nil; _, err = it.Next(&spec) {
		if prg != nil {
			filterInputs := map[string]interface{}{
				"name":                 spec.Name(),
				"project_id":           spec.ProjectID,
				"api_id":               spec.ApiID,
				"version_id":           spec.VersionID,
				"spec_id":              spec.SpecID,
				"filename":             spec.FileName,
				"description":          spec.Description,
				"create_time":          spec.CreateTime,
				"revision_create_time": spec.RevisionCreateTime,
				"revision_update_time": spec.RevisionUpdateTime,
				"mime_type":            spec.MimeType,
				"size_bytes":           spec.SizeInBytes,
				"source_uri":           spec.SourceURI,
			}
			filterInputs["labels"], err = spec.LabelsMap()
			if err != nil {
				return nil, internalError(err)
			}
			if out, _, err := prg.Eval(filterInputs); err != nil {
				return nil, invalidArgumentError(err)
			} else if v, ok := out.Value().(bool); !ok {
				return nil, invalidArgumentError(fmt.Errorf("expression does not evaluate to a boolean (instead yielding %T)", out.Value()))
			} else if !v {
				continue
			}
		}

		var specMessage *rpc.ApiSpec
		if req.GetView() == rpc.View_FULL {
			name, err := names.ParseSpecRevision(spec.RevisionName())
			if err != nil {
				continue
			}

			blob, err := getSpecRevisionContents(ctx, client, name)
			if err != nil {
				continue
			}

			specMessage, err = spec.FullMessage(blob, spec.Name())
			if err != nil {
				continue
			}
		} else {
			specMessage, err = spec.BasicMessage(spec.Name())
			if err != nil {
				continue
			}
		}

		specMessages = append(specMessages, specMessage)
		if len(specMessages) == pageSize {
			break
		}
	}
	if err != nil && err != iterator.Done {
		return nil, internalError(err)
	}
	responses := &rpc.ListApiSpecsResponse{
		ApiSpecs: specMessages,
	}
	responses.NextPageToken, err = it.GetCursor(len(specMessages))
	if err != nil {
		return nil, internalError(err)
	}
	return responses, nil
}

// UpdateApiSpec handles the corresponding API request.
func (s *RegistryServer) UpdateApiSpec(ctx context.Context, req *rpc.UpdateApiSpecRequest) (*rpc.ApiSpec, error) {
	client, err := s.getStorageClient(ctx)
	if err != nil {
		return nil, unavailableError(err)
	}
	defer s.releaseStorageClient(client)

	if req.GetApiSpec() == nil {
		return nil, invalidArgumentError(fmt.Errorf("invalid api_spec %+v: body must be provided", req.GetApiSpec()))
	}

	name, err := names.ParseSpec(req.ApiSpec.GetName())
	if err != nil {
		return nil, invalidArgumentError(err)
	}

	spec, err := getSpec(ctx, client, name)
	if req.GetAllowMissing() && isNotFound(err) {
		return s.createSpec(ctx, name, req.GetApiSpec())
	} else if err != nil {
		return nil, err
	}

	// Mark the current revision as non-current so the update becomes the only current revision.
	spec.Currency = models.NotCurrent
	if err := saveSpecRevision(ctx, client, spec); err != nil {
		return nil, err
	}

	// Apply the update to the spec - possibly changing the revision ID.
	if err := spec.Update(req.GetApiSpec(), req.GetUpdateMask()); err != nil {
		return nil, internalError(err)
	}

	// Save the updated/current spec. This creates a new revision or updates the previous one.
	if err := saveSpecRevision(ctx, client, spec); err != nil {
		return nil, err
	}

	// If the spec contents were updated, save a new blob.
	implicitUpdate := req.GetUpdateMask() == nil && len(req.ApiSpec.GetContents()) > 0
	explicitUpdate := len(fieldmaskpb.Intersect(req.GetUpdateMask(), &fieldmaskpb.FieldMask{Paths: []string{"contents"}}).GetPaths()) > 0
	if implicitUpdate || explicitUpdate {
		if err := saveSpecRevisionContents(ctx, client, spec, req.ApiSpec.GetContents()); err != nil {
			return nil, err
		}
	}

	message, err := spec.BasicMessage(name.String())
	if err != nil {
		return nil, internalError(err)
	}

	s.notify(rpc.Notification_UPDATED, spec.RevisionName())
	return message, nil
}

// fetchMostRecentNonCurrentRevisionOfSpec gets the most recent revision that's not current.
func (s *RegistryServer) fetchMostRecentNonCurrentRevisionOfSpec(ctx context.Context, client storage.Client, name names.Spec) (storage.Key, *models.Spec, error) {
	q := client.NewQuery(storage.SpecEntityName)
	q = q.Require("ProjectID", name.ProjectID)
	q = q.Require("ApiID", name.ApiID)
	q = q.Require("VersionID", name.VersionID)
	q = q.Require("SpecID", name.SpecID)
	q = q.Require("Currency", models.NotCurrent)
	q = q.Order("-CreateTime")
	it := client.Run(ctx, q)

	if s.weTrustTheSort {
		spec := &models.Spec{}
		k, err := it.Next(spec)
		if err != nil {
			return nil, nil, client.NotFoundError()
		}
		return k, spec, nil
	} else {
		specs := make([]*models.Spec, 0)
		for {
			spec := &models.Spec{}
			if _, err := it.Next(spec); err != nil {
				break
			}
			specs = append(specs, spec)
		}
		sort.Slice(specs, func(i, j int) bool {
			return specs[i].CreateTime.After(specs[j].CreateTime)
		})
		k := client.NewKey("Spec", specs[0].Key)
		return k, specs[0], nil
	}
}

func getSpec(ctx context.Context, client storage.Client, name names.Spec) (*models.Spec, error) {
	q := client.NewQuery(storage.SpecEntityName)
	q = q.Require("ProjectID", name.ProjectID)
	q = q.Require("ApiID", name.ApiID)
	q = q.Require("VersionID", name.VersionID)
	q = q.Require("SpecID", name.SpecID)
	q = q.Require("Currency", models.IsCurrent)
	it := client.Run(ctx, q)
	spec := &models.Spec{}
	if _, err := it.Next(spec); err != nil {
		return nil, notFoundError(err)
	}
	return spec, nil
}

func deleteSpec(ctx context.Context, client storage.Client, name names.Spec) error {
	if err := client.DeleteChildrenOfSpec(ctx, name); err != nil {
		return internalError(err)
	}

	k := client.NewKey(storage.SpecEntityName, name.String())
	if err := client.Delete(ctx, k); err != nil {
		return internalError(err)
	}

	q := client.NewQuery(storage.SpecEntityName)
	q = q.Require("ProjectID", name.ProjectID)
	q = q.Require("ApiID", name.ApiID)
	q = q.Require("VersionID", name.VersionID)
	q = q.Require("SpecID", name.SpecID)
	if err := client.DeleteAllMatches(ctx, q); err != nil {
		return internalError(err)
	}

	return nil
}
