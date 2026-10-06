// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"

	organization_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service/client/organizations"
)

// OrgServiceAdapter maps the shared org-service client onto OrgService.
type OrgServiceAdapter struct {
	Client *organization_service.Client
}

// GetOrganization returns ErrOrgNotFound on 404.
func (a OrgServiceAdapter) GetOrganization(ctx context.Context, orgID string) (*Org, error) {
	org, err := a.Client.GetOrganization(ctx, orgID)
	if err != nil {
		var notFound *organizations.GetOrgNotFound
		if errors.As(err, &notFound) {
			return nil, ErrOrgNotFound
		}
		return nil, err
	}
	if org == nil {
		return nil, ErrOrgNotFound
	}
	return &Org{ID: org.ID, Name: org.Name, Website: org.Link, SigningEntityNames: org.SigningEntityName}, nil
}

// LookupOrganization finds one Account by name or website domain (ErrOrgNotFound when none).
func (a OrgServiceAdapter) LookupOrganization(ctx context.Context, name, domain string) (*Org, error) {
	var namePtr, domainPtr *string
	if name != "" {
		namePtr = &name
	}
	if domain != "" {
		domainPtr = &domain
	}
	res, err := a.Client.SearchOrgLookup(ctx, namePtr, domainPtr)
	if err != nil {
		var notFound *organizations.LookupNotFound
		if errors.As(err, &notFound) {
			return nil, ErrOrgNotFound
		}
		return nil, err
	}
	if res == nil || res.Payload == nil || res.Payload.ID == "" {
		return nil, ErrOrgNotFound
	}
	return &Org{ID: res.Payload.ID, Name: res.Payload.Name, Website: res.Payload.Link}, nil
}

// CreateUserRoleScope grants a role scope by LFID username; an existing grant is not an error.
func (a OrgServiceAdapter) CreateUserRoleScope(ctx context.Context, username, organizationID, objectType, objectID, roleID string) error {
	return a.Client.CreateOrgUserRoleScopeByUsername(ctx, username, organizationID, objectType, objectID, roleID)
}

// DeleteUserRoleScope removes one grant (ACS grant id); a missing grant is success.
func (a OrgServiceAdapter) DeleteUserRoleScope(ctx context.Context, organizationID, roleID, grantID, username string) error {
	email := ""
	err := a.Client.DeleteOrgUserRoleOrgScopeProjectOrg(ctx, organizationID, roleID, grantID, &username, &email)
	if err != nil {
		var notFound *organizations.DeleteOrgUsrRoleScopesNotFound
		if errors.As(err, &notFound) {
			return nil
		}
		return err
	}
	return nil
}

// DynamoECLACounter counts rows of the signature-user-ccla-company-index partition of a company.
type DynamoECLACounter struct {
	DB    dynamodbiface.DynamoDBAPI
	Table string
}

// CountECLAs sums Count over all pages.
func (c DynamoECLACounter) CountECLAs(ctx context.Context, companyID string) (int, error) {
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(c.Table),
		IndexName:                 aws.String("signature-user-ccla-company-index"),
		KeyConditionExpression:    aws.String("signature_user_ccla_company_id = :id"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{":id": {S: aws.String(companyID)}},
		Select:                    aws.String(dynamodb.SelectCount),
	}
	total := 0
	for {
		out, err := c.DB.QueryWithContext(ctx, input)
		if err != nil {
			return 0, fmt.Errorf("counting ECLAs of %s: %w", companyID, err)
		}
		total += int(aws.Int64Value(out.Count))
		if len(out.LastEvaluatedKey) == 0 {
			return total, nil
		}
		input.ExclusiveStartKey = out.LastEvaluatedKey
	}
}
