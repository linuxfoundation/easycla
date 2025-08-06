// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package approval_list

import (
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/aws/aws-sdk-go/service/dynamodb/expression"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
)

// buildCclaWhitelistRequestsModels builds the request models
func buildCclaWhitelistRequestsModels(results *dynamodb.QueryOutput) ([]models.CclaWhitelistRequest, error) {
	requests := make([]models.CclaWhitelistRequest, 0)

	var itemRequests []CclaWhitelistRequest

	err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &itemRequests)
	if err != nil {
		log.Warnf("error unmarshalling CCLA Authorization Request from database, error: %v",
			err)
		return nil, err
	}
	for _, r := range itemRequests {
		requests = append(requests, models.CclaWhitelistRequest{
			CompanyID:          r.CompanyID,
			CompanyName:        r.CompanyName,
			DateCreated:        r.DateCreated,
			DateModified:       r.DateModified,
			ProjectID:          r.ProjectID,
			ProjectName:        r.ProjectName,
			RequestID:          r.RequestID,
			RequestStatus:      r.RequestStatus,
			UserEmails:         r.UserEmails,
			UserGithubID:       r.UserGithubID,
			UserGithubUsername: r.UserGithubUsername,
			UserID:             r.UserID,
			UserName:           r.UserName,
			Version:            r.Version,
		})
	}
	return requests, nil
}

// addStringAttribute adds the specified attribute as a string
func addStringAttribute(item map[string]*dynamodb.AttributeValue, key string, value string) {
	if value != "" {
		item[key] = &dynamodb.AttributeValue{S: aws.String(value)}
	}
}

// addStringSliceAttribute adds the specified attribute as a string slice
func addStringSliceAttribute(item map[string]*dynamodb.AttributeValue, key string, value []string) {
	if len(value) > 0 {
		item[key] = &dynamodb.AttributeValue{SS: aws.StringSlice(value)}
	}
}

// addConditionToFilter - helper routine for adding a filter condition
func addConditionToFilter(filter expression.ConditionBuilder, cond expression.ConditionBuilder, filterAdded *bool) expression.ConditionBuilder {
	if !(*filterAdded) {
		*filterAdded = true
		filter = cond
	} else {
		filter = filter.And(cond)
	}
	return filter
}
