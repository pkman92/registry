// Copyright 2022 Google LLC. All Rights Reserved.
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

package storage

import (
	"context"

	"github.com/apigee/registry/server/registry/internal/storage/models"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"gorm.io/gorm"
)

func (c *Client) SaveProject(ctx context.Context, v *models.Project) error {
	v.Key = v.Name()
	return c.save(v)
}

func (c *Client) SaveApi(ctx context.Context, v *models.Api) error {
	v.Key = v.Name()
	return c.save(v)
}

func (c *Client) SaveVersion(ctx context.Context, v *models.Version) error {
	v.Key = v.Name()
	return c.save(v)
}

func (c *Client) SaveSpecRevision(ctx context.Context, v *models.Spec) error {
	v.Key = v.RevisionName()
	return c.save(v)
}

func (c *Client) SaveSpecRevisionContents(ctx context.Context, spec *models.Spec, contents []byte) error {
	v := models.NewBlobForSpec(spec, contents)
	v.Key = spec.RevisionName()
	return c.save(v)
}

func (c *Client) SaveSpecRevisionTag(ctx context.Context, v *models.SpecRevisionTag) error {
	v.Key = v.String()
	return c.save(v)
}

func (c *Client) SaveDeploymentRevision(ctx context.Context, v *models.Deployment) error {
	v.Key = v.RevisionName()
	return c.save(v)
}

func (c *Client) SaveDeploymentRevisionTag(ctx context.Context, v *models.DeploymentRevisionTag) error {
	v.Key = v.String()
	return c.save(v)
}

func (c *Client) SaveArtifact(ctx context.Context, v *models.Artifact) error {
	v.Key = v.Name()
	return c.save(v)
}

func (c *Client) SaveArtifactContents(ctx context.Context, artifact *models.Artifact, contents []byte) error {
	v := models.NewBlobForArtifact(artifact, contents)
	v.Key = artifact.Name()
	return c.save(v)
}

func (c *Client) save(v interface{}) error {
	err := c.db.Transaction(func(tx *gorm.DB) error {
		// Update all fields from model: https://gorm.io/docs/update.html#Update-Selected-Fields
		got := tx.Model(v).Select("*").Updates(v)
		if err := got.Error; err != nil {
			return got.Error
		}

		if got.RowsAffected == 0 {
			if err := tx.Create(v).Error; err != nil {
				return err
			}
		}

		return nil
	})
	if err != nil {
		return status.Error(codes.Internal, err.Error())
	}

	return nil
}
