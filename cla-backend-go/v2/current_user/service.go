// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package current_user

import (
	"context"
	"fmt"

	"github.com/jinzhu/copier"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	v2Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

type service struct{}

// Service functions for current_user
type Service interface {
	UserFromContext(ctx context.Context) (*v2Models.User, error)
}

// NewService returns instance of current_user service
func NewService() Service {
	return &service{}
}

func (s *service) UserFromContext(ctx context.Context) (*v2Models.User, error) {
	f := logrus.Fields{
		"functionName":   "v2.current_user.service.UserFromContext",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	ctxUserModel, ok := ctx.Value("user").(*v1Models.User)
	if !ok || ctxUserModel == nil {
		msg := "unable to lookup user from context"
		log.WithFields(f).Warn(msg)
		return nil, fmt.Errorf("cannot find user data in context")
	}

	var v2UserModel v2Models.User
	copyErr := copier.Copy(&v2UserModel, &ctxUserModel)
	if copyErr != nil {
		log.WithFields(f).Warnf("problem converting DB user model to a v2 user model, error: %+v", copyErr)
		return nil, copyErr
	}

	return &v2UserModel, nil
}
