// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package project

import (
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/jinzhu/copier"
)

func v1ProjectModel(in *models.ClaGroup) (*v1Models.ClaGroup, error) {
	out := &v1Models.ClaGroup{}
	err := copier.Copy(out, in)
	if err != nil {
		return nil, err
	}
	return out, nil
}

func v2ProjectModel(in *v1Models.ClaGroup) (*models.ClaGroup, error) {
	out := &models.ClaGroup{}
	err := copier.Copy(out, in)
	if err != nil {
		return nil, err
	}
	return out, nil
}
