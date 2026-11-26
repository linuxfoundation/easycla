// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package orgimport

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"

	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
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

// LatestECLA returns the newest day of the signed rows of the same partition.
func (c DynamoECLACounter) LatestECLA(ctx context.Context, companyID string) (string, error) {
	dates := []string{"signed_on", "user_docusign_date_signed", "date_created", "approx_date_created", "date_modified"}
	input := &dynamodb.QueryInput{
		TableName:                 aws.String(c.Table),
		IndexName:                 aws.String("signature-user-ccla-company-index"),
		KeyConditionExpression:    aws.String("signature_user_ccla_company_id = :id"),
		FilterExpression:          aws.String("signature_signed = :signed"),
		ExpressionAttributeValues: map[string]*dynamodb.AttributeValue{":id": {S: aws.String(companyID)}, ":signed": {BOOL: aws.Bool(true)}},
		ProjectionExpression:      aws.String(strings.Join(dates, ", ")),
	}
	latest := ""
	for {
		out, err := c.DB.QueryWithContext(ctx, input)
		if err != nil {
			return "", fmt.Errorf("reading the ECLA dates of %s: %w", companyID, err)
		}
		for _, item := range out.Items {
			values := make([]string, 0, len(dates))
			for _, name := range dates {
				if v := item[name]; v != nil {
					values = append(values, aws.StringValue(v.S))
				}
			}
			if d := signedDay(values...); d > latest {
				latest = d
			}
		}
		if len(out.LastEvaluatedKey) == 0 {
			return latest, nil
		}
		input.ExclusiveStartKey = out.LastEvaluatedKey
	}
}

// MemberAccounts is the member-service find-or-create (member_service.Client satisfies it).
type MemberAccounts interface {
	FindOrCreateB2BOrg(ctx context.Context, name, website string) (*member_service.B2BOrg, member_service.Action, error)
}

// MemberApex creates Accounts through member-service, the path of the company create flow; it has no dry run.
type MemberApex struct {
	Members MemberAccounts
}

// FindOrCreate maps the member-service answer onto the Apex contract; several Accounts on the website domain is ambiguous.
func (m MemberApex) FindOrCreate(ctx context.Context, req ApexRequest) (*ApexResult, error) {
	if req.DryRun {
		return nil, errors.New("member-service find-or-create has no dry run")
	}
	org, action, err := m.Members.FindOrCreateB2BOrg(ctx, req.Name, req.Website)
	if errors.Is(err, member_service.ErrAmbiguousDomain) {
		return &ApexResult{Action: ActionAmbiguous}, nil
	}
	if err != nil {
		return nil, err
	}
	res := &ApexResult{ID: org.UID, Action: string(action)}
	if (res.Action != ActionMatched && res.Action != ActionCreated) || !IsSFID(res.ID) {
		return nil, fmt.Errorf("member-service find-or-create: unusable answer %q %q", res.Action, res.ID)
	}
	return res, nil
}
