package interfaces

import (
	"context"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	gitlabModels "github.com/communitybridge/easycla/cla-backend-go/v2/gitlab-activity/models"
	"github.com/sirupsen/logrus"
	"github.com/xanzy/go-gitlab"
)

type GitlabActivityService interface {
	ProcessMergeCommentActivity(ctx context.Context, secretToken string, commentEvent *gitlab.MergeEvent) error
	ProcessMergeOpenedActivity(ctx context.Context, secretToken string, mergeEvent *gitlab.MergeEvent) error
	ProcessMergeActivity(ctx context.Context, secretToken string, input *gitlabModels.ProcessMergeActivityInput) error
	IsUserApprovedForSignature(ctx context.Context, f logrus.Fields, corporateSignature *models.Signature, user *models.User, gitlabUser *gitlab.User) bool
	HasUserSigned(ctx context.Context, claGroupID string, gitlabUser *gitlab.User) (bool, error)
}
