// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package events

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbattribute"
	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/aws/aws-sdk-go/service/dynamodb/expression"

	"github.com/aws/aws-sdk-go/aws"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"

	"github.com/gofrs/uuid"

	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/dynamodb"

	"github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	eventOps "github.com/communitybridge/easycla/cla-backend-go/gen/v1/restapi/operations/events"
)

// errors
var (
	ErrUserIDRequired    = errors.New("UserID cannot be empty")    //nolint
	ErrEventTypeRequired = errors.New("EventType cannot be empty") //nolint
)

// indexes
const (
	CompanySFIDFoundationSFIDEpochIndex = "company-sfid-foundation-sfid-event-time-epoch-index"
	CompanySFIDProjectIDEpochIndex      = "company-sfid-project-id-event-time-epoch-index"
	CompanyIDEventTypeIndex             = "company-id-event-type-index"
	EventFoundationSFIDEpochIndex       = "event-foundation-sfid-event-time-epoch-index"
	EventProjectIDEpochIndex            = "event-project-id-event-time-epoch-index"
	EventCLAGroupIDEpochIndex           = "event-cla-group-id-event-time-epoch-index"
	EventCompanySFIDEventDataLowerIndex = "event-company-sfid-event-data-lower-index"
)

// constants
const (
	HugePageSize    = 10000
	DefaultPageSize = 10
)

// Repository interface defines methods of event repository service
type Repository interface {
	CreateEvent(event *models.Event) error
	AddDataToEvent(eventID, parentProjectSFID, projectSFID, projectSFName, companySFID, projectID string) error
	SearchEvents(params *eventOps.SearchEventsParams, pageSize int64) (*models.EventList, error)
	GetRecentEvents(pageSize int64) (*models.EventList, error)

	GetCompanyFoundationEvents(companySFID, companyID, foundationSFID string, nextKey *string, paramPageSize *int64, searchTerm *string, all bool) (*models.EventList, error)
	GetCompanyClaGroupEvents(companySFID, companyID, claGroupID string, nextKey *string, paramPageSize *int64, searchTerm *string, all bool) (*models.EventList, error)
	GetCompanyEvents(companyID, eventType string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error)
	GetFoundationEvents(foundationSFID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error)
	GetClaGroupEvents(claGroupID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error)
}

// repository data model
type repository struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
	eventsTable    string
}

// NewRepository creates a new instance of the event repository
func NewRepository(awsSession *session.Session, stage string) Repository {
	return &repository{
		stage:          stage,
		dynamoDBClient: dynamodb.New(awsSession),
		eventsTable:    fmt.Sprintf("cla-%s-events", stage),
	}
}

func toDateFormat(t time.Time) string {
	//DD-MM-YYYY format
	return t.Format("02-01-2006")
}

// CreateEvent event will create event in database.
func (repo *repository) CreateEvent(event *models.Event) error {
	f := logrus.Fields{
		"functionName": "v1.events.repository.CreateEvent",
	}

	if event.UserID == "" {
		return ErrUserIDRequired
	}
	if event.EventType == "" {
		return ErrEventTypeRequired
	}
	eventID, err := uuid.NewV4()
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to generate a UUID for a whitelist request, error: %v", err)
		return err
	}

	currentTime, currentTimeString := utils.CurrentTime()
	input := &dynamodb.PutItemInput{
		Item:      map[string]*dynamodb.AttributeValue{},
		TableName: aws.String(repo.eventsTable),
	}

	eventDateAndContainsPII := fmt.Sprintf("%s#%t", toDateFormat(currentTime), event.ContainsPII)
	addAttribute(input.Item, "event_id", eventID.String())
	addAttribute(input.Item, "event_type", event.EventType)

	addAttribute(input.Item, "event_user_id", event.UserID)
	addAttribute(input.Item, "event_user_name", event.UserName)
	addAttribute(input.Item, "event_user_name_lower", strings.ToLower(event.UserName))
	addAttribute(input.Item, "event_lf_username", event.LfUsername)

	addAttribute(input.Item, "event_company_id", event.EventCompanyID)
	addAttribute(input.Item, "event_company_sfid", event.EventCompanySFID)
	addAttribute(input.Item, "event_company_name", event.EventCompanyName)
	addAttribute(input.Item, "event_company_name_lower", strings.ToLower(event.EventCompanyName))

	addAttribute(input.Item, "event_cla_group_id", event.EventCLAGroupID)
	addAttribute(input.Item, "event_cla_group_name", event.EventCLAGroupName)
	addAttribute(input.Item, "event_cla_group_name_lower", strings.ToLower(event.EventCLAGroupName))

	addAttribute(input.Item, "event_project_id", event.EventProjectID)
	addAttribute(input.Item, "event_project_sfid", event.EventProjectSFID)
	addAttribute(input.Item, "event_project_name", event.EventProjectName)
	addAttribute(input.Item, "event_project_name_lower", strings.ToLower(event.EventProjectName))
	addAttribute(input.Item, "event_parent_project_sfid", event.EventParentProjectSFID)
	addAttribute(input.Item, "event_parent_project_name", event.EventParentProjectName)

	addAttribute(input.Item, "event_data", event.EventData)
	// For filtering/searching
	addAttribute(input.Item, "event_data_lower", strings.ToLower(event.EventData))
	addAttribute(input.Item, "event_summary", event.EventSummary)

	addAttribute(input.Item, "event_time", currentTimeString)
	addAttribute(input.Item, "event_date", toDateFormat(currentTime))
	addAttribute(input.Item, "event_date_and_contains_pii", eventDateAndContainsPII)
	addAttribute(input.Item, "date_created", toDateFormat(currentTime))
	addAttribute(input.Item, "date_modified", toDateFormat(currentTime))

	input.Item["contains_pii"] = &dynamodb.AttributeValue{BOOL: &event.ContainsPII}
	input.Item["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(currentTime.Unix(), 10))}
	if event.EventCompanyID != "" && event.EventProjectSFID != "" {
		companyIDExternalProjectID := fmt.Sprintf("%s#%s", event.EventCompanyID, event.EventProjectSFID)
		addAttribute(input.Item, "company_id_external_project_id", companyIDExternalProjectID)
	}

	_, err = repo.dynamoDBClient.PutItem(input)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to create a new event, error: %v", err)
		return err
	}
	log.WithFields(f).Infof("added event ID: %s of type: %s", eventID.String(), event.EventType)

	return nil
}

func addAttribute(item map[string]*dynamodb.AttributeValue, key string, value string) {
	if value != "" {
		item[key] = &dynamodb.AttributeValue{S: aws.String(value)}
	}
}

func addConditionToFilter(filter expression.ConditionBuilder, cond expression.ConditionBuilder, filterAdded *bool) expression.ConditionBuilder {
	if !(*filterAdded) {
		*filterAdded = true
		filter = cond
	} else {
		filter = filter.And(cond)
	}
	return filter
}

// createSearchEventFilter creates the search event filter
func createSearchEventFilter(pk string, sk string, params *eventOps.SearchEventsParams) *expression.ConditionBuilder {
	var filter expression.ConditionBuilder
	var filterAdded bool
	if params.ProjectID != nil && "event_project_id" != pk && "event_project_id" != sk { //nolint
		filterExpression := expression.Name("event_project_id").Equal(expression.Value(params.ProjectID))
		filter = addConditionToFilter(filter, filterExpression, &filterAdded)
	}
	if params.CompanyID != nil && "event_company_id" != pk && "event_company_id" != sk { //nolint
		filterExpression := expression.Name("event_company_id").Equal(expression.Value(params.CompanyID))
		filter = addConditionToFilter(filter, filterExpression, &filterAdded)
	}
	if params.UserID != nil && "event_user_id" != pk && "event_user_id" != sk { //nolint
		filterExpression := expression.Name("event_user_id").Equal(expression.Value(params.UserID))
		filter = addConditionToFilter(filter, filterExpression, &filterAdded)
	}
	if params.EventType != nil && "event_type" != pk && "event_type" != sk { //nolint
		filterExpression := expression.Name("event_type").Equal(expression.Value(params.EventType))
		filter = addConditionToFilter(filter, filterExpression, &filterAdded)
	}
	if params.After != nil && "event_time_epoch" != pk && "event_time_epoch" != sk { //nolint
		filterExpression := expression.Name("event_time_epoch").GreaterThanEqual(expression.Value(params.After))
		filter = addConditionToFilter(filter, filterExpression, &filterAdded)
	}
	if params.Before != nil && "event_time_epoch" != pk && "event_time_epoch" != sk { //nolint
		filterExpression := expression.Name("event_time_epoch").LessThanEqual(expression.Value(params.Before))
		filter = addConditionToFilter(filter, filterExpression, &filterAdded)
	}
	if params.UserName != nil && "user_name_lower" != pk && "user_name_lower" != sk { //nolint
		filterExpression := expression.Name("user_name_lower").Contains(strings.ToLower(*params.UserName))
		filter = addConditionToFilter(filter, filterExpression, &filterAdded)
	}
	if params.CompanyName != nil && "event_company_name_lower" != pk && "event_company_name_lower" != sk { //nolint
		filterExpression := expression.Name("event_company_name_lower").Contains(strings.ToLower(*params.CompanyName))
		filter = addConditionToFilter(filter, filterExpression, &filterAdded)
	}
	if params.SearchTerm != nil {
		filterExpression := expression.Name("event_data_lower").Contains(strings.ToLower(*params.SearchTerm))
		filter = addConditionToFilter(filter, filterExpression, &filterAdded)
	}
	if filterAdded {
		return &filter
	}
	return nil
}

// addTimeExpression adds the time expression to the query
func addTimeExpression(keyCond expression.KeyConditionBuilder, params *eventOps.SearchEventsParams) expression.KeyConditionBuilder {
	if params.Before != nil && params.After != nil {
		exp := expression.Key("event_time_epoch").Between(expression.Value(params.After), expression.Value(params.Before))
		return keyCond.And(exp)
	}
	if params.After != nil {
		exp := expression.Key("event_time_epoch").GreaterThanEqual(expression.Value(params.After))
		return keyCond.And(exp)
	}
	if params.Before != nil {
		exp := expression.Key("event_time_epoch").LessThanEqual(expression.Value(params.Before))
		return keyCond.And(exp)
	}
	return keyCond
}

// SearchEvents returns list of events matching with filter criteria.
func (repo *repository) SearchEvents(params *eventOps.SearchEventsParams, pageSize int64) (*models.EventList, error) {
	f := logrus.Fields{
		"functionName": "v1.events.repository.SearchEvents",
		"pageSize":     pageSize,
	}

	if params.ProjectID == nil {
		return nil, errors.New("invalid request. projectID is compulsory")
	}
	var condition expression.KeyConditionBuilder
	var indexName, pk, sk string
	builder := expression.NewBuilder().WithProjection(buildProjection())

	switch {
	case params.ProjectID != nil:
		// search by projectID
		indexName = "event-project-id-event-time-epoch-index"
		condition = expression.Key("event_project_id").Equal(expression.Value(params.ProjectID))
		pk = "event_project_id"
		condition = addTimeExpression(condition, params)
		sk = "event_time_epoch"
	}
	filter := createSearchEventFilter(pk, sk, params)
	if filter != nil {
		builder = builder.WithFilter(*filter)
	}

	builder = builder.WithKeyCondition(condition)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		return nil, err
	}
	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.eventsTable),
		IndexName:                 aws.String(indexName),
		Limit:                     aws.Int64(pageSize), // The maximum number of items to evaluate (not necessarily the number of matching items)
	}
	if params.SortOrder != nil && *params.SortOrder == "desc" {
		queryInput.ScanIndexForward = aws.Bool(false)
	}

	if params.NextKey != nil {
		queryInput.ExclusiveStartKey, err = decodeNextKey(*params.NextKey)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem decoding next key value")
			return nil, err
		}
		log.WithFields(f).Debugf("received a nextKey, value: %s - decoded: %+v", *params.NextKey, queryInput.ExclusiveStartKey)
	}

	events := make([]*models.Event, 0)
	var results *dynamodb.QueryOutput

	for {
		// Perform the query...
		var errQuery error
		results, errQuery = repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).WithError(errQuery).Warnf("error retrieving events")
			return nil, errQuery
		}

		// Build the result models
		eventsList, modelErr := buildEventListModels(results)
		if modelErr != nil {
			log.WithFields(f).WithError(modelErr).Warn("error convert event list models")
			return nil, modelErr
		}

		// Trim to how many the caller asked for - just in case we go over
		events = append(events, eventsList...)
		if int64(len(events)) > pageSize {
			events = events[:pageSize]
		}
		log.WithFields(f).Debugf("loaded %d events", len(events))

		// We have more records if last evaluated key has a value
		log.WithFields(f).Debugf("last evaluated key %+v", results.LastEvaluatedKey)
		if len(results.LastEvaluatedKey) > 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}

		if int64(len(events)) >= pageSize {
			break
		}
	}

	response := &models.EventList{
		Events: events,
	}

	log.WithFields(f).Debugf("returning %d events - last key: %+v", len(events), results.LastEvaluatedKey)
	if len(results.LastEvaluatedKey) > 0 {
		log.WithFields(f).Debug("building next key...")
		encodedString, err := buildNextKey(indexName, events[len(events)-1])
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to build nextKey")
		}
		response.NextKey = encodedString
		log.WithFields(f).Debugf("lastEvaluatedKey encoded is: %s", encodedString)
	}

	return response, nil
}

// queryEventsTable queries events table on index
func (repo *repository) queryEventsTable(indexName string, condition expression.KeyConditionBuilder, filter *expression.ConditionBuilder, nextKey *string, pageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	f := logrus.Fields{
		"functionName": "v1.events.repository.queryEventsTable",
		"indexName":    indexName,
		//"nextKey":      aws.StringValue(nextKey),
		"pageSize":   aws.Int64Value(pageSize),
		"all":        all,
		"searchTerm": aws.StringValue(searchTerm),
	}

	log.WithFields(f).Debug("querying events table...")
	builder := expression.NewBuilder().WithKeyCondition(condition)

	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem building events query")
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		TableName:                 aws.String(repo.eventsTable),
		IndexName:                 aws.String(indexName),
		ScanIndexForward:          aws.Bool(false), // Specifies the order for index traversal: If true (default), the traversal is performed in ascending order; if false, the traversal is performed in descending order.
	}

	if filter != nil {
		queryInput.FilterExpression = expr.Filter()
	}

	if all {
		queryInput.Limit = aws.Int64(HugePageSize)
	} else {
		if pageSize == nil {
			queryInput.Limit = aws.Int64(DefaultPageSize)
		} else {
			if *pageSize > HugePageSize {
				queryInput.Limit = aws.Int64(HugePageSize)
			}
			queryInput.Limit = pageSize
		}
	}
	maxResults := *queryInput.Limit

	// If we have the next key, set the exclusive start key value
	if nextKey != nil {
		queryInput.ExclusiveStartKey, err = decodeNextKey(*nextKey)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem decoding next key value")
			return nil, err
		}
		log.WithFields(f).Debugf("received a nextKey, value: %s - decoded: %+v", *nextKey, queryInput.ExclusiveStartKey)
	}

	events := make([]*models.Event, 0)
	var results *dynamodb.QueryOutput

	for {
		// Perform the query...
		var errQuery error
		results, errQuery = repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).WithError(errQuery).Warn("error retrieving events")
			return nil, errQuery
		}

		// Build the result models
		eventsList, modelErr := buildEventListModels(results)
		if modelErr != nil {
			log.WithFields(f).WithError(modelErr).Warn("error convert event list models")
			return nil, modelErr
		}

		events = append(events, eventsList...)
		// Add search term filtering
		if len(events) > 0 && searchTerm != nil {
			log.WithFields(f).Debugf("filtering events by search term: %s", *searchTerm)
			events = filterEventsBySearchTerm(events, *searchTerm)
		}

		// Trim to how many the caller asked for - just in case we go over
		if int64(len(events)) > maxResults {
			events = events[:maxResults]
		}
		log.WithFields(f).Debugf("loaded %d events", len(events))

		// We have more records if last evaluated key has a value
		log.WithFields(f).Debugf("last evaluated key %+v", results.LastEvaluatedKey)
		if len(results.LastEvaluatedKey) > 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}

		if int64(len(events)) >= maxResults {
			break
		}
	}

	if len(events) > 0 {
		response := &models.EventList{
			Events:      events,
			ResultCount: int64(len(events)),
		}
		log.WithFields(f).Debugf("returning %d events - last key: %+v", len(events), results.LastEvaluatedKey)
		if len(results.LastEvaluatedKey) > 0 {
			log.WithFields(f).Debug("building next key...")
			encodedString, err := buildNextKey(indexName, events[len(events)-1])
			if err != nil {
				log.WithFields(f).WithError(err).Warn("unable to build nextKey")
			}
			response.NextKey = encodedString
			log.WithFields(f).Debugf("lastEvaluatedKey encoded is: %s", encodedString)
		}

		return response, nil
	}

	// Just return an empty response - no events - just an empty list, and no nextKey
	return &models.EventList{
		Events:      []*models.Event{},
		ResultCount: 0,
	}, nil
}

func filterEventsBySearchTerm(events []*models.Event, s string) []*models.Event {
	var filteredEvents []*models.Event
	for _, event := range events {
		log.Debugf("checking event: %s", event.EventData)
		if strings.Contains(strings.ToLower(event.EventData), strings.ToLower(s)) {
			filteredEvents = append(filteredEvents, event)
		}
	}
	return filteredEvents
}

func buildNextKey(indexName string, event *models.Event) (string, error) {
	nextKey := make(map[string]*dynamodb.AttributeValue)
	nextKey["event_id"] = &dynamodb.AttributeValue{S: aws.String(event.EventID)}
	switch indexName {
	case CompanySFIDFoundationSFIDEpochIndex:
		nextKey["company_sfid_foundation_sfid"] = &dynamodb.AttributeValue{
			S: aws.String(fmt.Sprintf("%s#%s", event.EventCompanySFID, event.EventParentProjectSFID)),
		}
		nextKey["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(event.EventTimeEpoch, 10))}
	case CompanySFIDProjectIDEpochIndex:
		nextKey["company_sfid_project_id"] = &dynamodb.AttributeValue{
			S: aws.String(fmt.Sprintf("%s#%s", event.EventCompanySFID, event.EventProjectID)),
		}
		nextKey["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(event.EventTimeEpoch, 10))}
	case EventFoundationSFIDEpochIndex:
		nextKey["event_parent_project_sfid"] = &dynamodb.AttributeValue{S: aws.String(event.EventParentProjectSFID)}
		nextKey["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(event.EventTimeEpoch, 10))}
	case EventProjectIDEpochIndex:
		nextKey["event_project_id"] = &dynamodb.AttributeValue{S: aws.String(event.EventProjectID)}
		nextKey["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(event.EventTimeEpoch, 10))}
	case CompanyIDEventTypeIndex:
		nextKey["company_id"] = &dynamodb.AttributeValue{S: aws.String(event.EventCompanyID)}
		nextKey["event_type"] = &dynamodb.AttributeValue{S: aws.String(event.EventType)}
	case EventCLAGroupIDEpochIndex:
		nextKey["event_cla_group_id"] = &dynamodb.AttributeValue{S: aws.String(event.EventCLAGroupID)}
		nextKey["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(event.EventTimeEpoch, 10))}
	}

	return encodeNextKey(nextKey)
}

// GetCompanyFoundationEvents returns the list of events for foundation and company
func (repo *repository) GetCompanyFoundationEvents(companySFID, companyID, foundationSFID string, nextKey *string, paramPageSize *int64, searchTerm *string, all bool) (*models.EventList, error) {
	f := logrus.Fields{
		"functionName":   "v1.events.repository.GetCompanyFoundationEvents",
		"companySFID":    companySFID,
		"companyID":      companyID,
		"foundationSFID": foundationSFID,
		"nextKey":        utils.StringValue(nextKey),
		"paramPageSize":  utils.Int64Value(paramPageSize),
		"loadAll":        all,
	}
	log.WithFields(f).Debugf("adding key condition of 'event_company_sfid_sfid = %s'", companySFID)
	keyCondition := expression.Key("event_company_sfid").Equal(expression.Value(companySFID))
	var filter expression.ConditionBuilder
	log.WithFields(f).Debugf("adding filter condition of 'event_parent_project_sfid = %s'", foundationSFID)
	filter = expression.Name("event_parent_project_sfid").Equal(expression.Value(foundationSFID))
	return repo.queryEventsTable(EventCompanySFIDEventDataLowerIndex, keyCondition, &filter, nextKey, paramPageSize, all, searchTerm)
}

// GetCompanyClaGroupEvents returns the list of events for cla group and the company
func (repo *repository) GetCompanyClaGroupEvents(companySFID, companyID, claGroupID string, nextKey *string, paramPageSize *int64, searchTerm *string, all bool) (*models.EventList, error) {
	f := logrus.Fields{
		"functionName":  "v1.events.repository.GetCompanyClaGroupEvents",
		"companySFID":   companySFID,
		"companyID":     companyID,
		"claGroupID":    claGroupID,
		"nextKey":       utils.StringValue(nextKey),
		"paramPageSize": utils.Int64Value(paramPageSize),
		"loadAll":       all,
	}
	log.WithFields(f).Debugf("adding key condition of 'event_cla_group_id = %s'", claGroupID)
	keyCondition := expression.Key("event_cla_group_id").Equal(expression.Value(claGroupID))
	var filter expression.ConditionBuilder
	log.WithFields(f).Debugf("adding filter condition of 'event_company_sfid = %s'", companySFID)
	filter = expression.Name("event_company_sfid").Equal(expression.Value(companySFID))
	return repo.queryEventsTable(EventCLAGroupIDEpochIndex, keyCondition, &filter, nextKey, paramPageSize, all, searchTerm)
}

// GetCompanyEvents returns the list of events for given company id and event types
func (repo *repository) GetCompanyEvents(companyID, eventType string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error) {
	f := logrus.Fields{
		"functionName":  "v1.events.repository.GetCompanyEvents",
		"companyID":     companyID,
		"nextKey":       utils.StringValue(nextKey),
		"paramPageSize": utils.Int64Value(paramPageSize),
		"loadAll":       all,
	}
	log.WithFields(f).Debugf("adding key condition of 'company_id = %s'", companyID)
	keyCondition := expression.Key("company_id").Equal(expression.Value(companyID)).And(
		expression.Key("event_type").Equal(expression.Value(eventType)))

	return repo.queryEventsTable(CompanyIDEventTypeIndex, keyCondition, nil, nextKey, paramPageSize, all, nil)
}

// GetFoundationEvents returns the list of foundation events
func (repo *repository) GetFoundationEvents(foundationSFID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	f := logrus.Fields{
		"functionName":   "v1.events.repository.GetFoundationEvents",
		"foundationSFID": foundationSFID,
		"nextKey":        utils.StringValue(nextKey),
		"paramPageSize":  utils.Int64Value(paramPageSize),
		"loadAll":        all,
		"searchTerm":     utils.StringValue(searchTerm),
	}
	log.WithFields(f).Debugf("adding key condition of 'event_parent_project_sfid = %s'", foundationSFID)
	keyCondition := expression.Key("event_parent_project_sfid").Equal(expression.Value(foundationSFID))
	return repo.queryEventsTable(EventFoundationSFIDEpochIndex, keyCondition, nil, nextKey, paramPageSize, all, searchTerm)
}

// GetClaGroupEvents returns the list of cla-group events
func (repo *repository) GetClaGroupEvents(claGroupID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	f := logrus.Fields{
		"functionName":  "v1.events.repository.GetClaGroupEvents",
		"claGroupID":    claGroupID,
		"nextKey":       utils.StringValue(nextKey),
		"paramPageSize": utils.Int64Value(paramPageSize),
		"loadAll":       all,
		"searchTerm":    utils.StringValue(searchTerm),
	}
	log.WithFields(f).Debugf("adding key condition of 'event_cla_group_id = %s'", claGroupID)
	keyCondition := expression.Key("event_cla_group_id").Equal(expression.Value(claGroupID))
	return repo.queryEventsTable(EventCLAGroupIDEpochIndex, keyCondition, nil, nextKey, paramPageSize, all, searchTerm)
}

// encodeNextKey encodes the map as a string
func encodeNextKey(in map[string]*dynamodb.AttributeValue) (string, error) {
	if len(in) == 0 {
		return "", nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// decodeNextKey decodes the next key value into a dynamodb attribute value
func decodeNextKey(str string) (map[string]*dynamodb.AttributeValue, error) {
	f := logrus.Fields{
		"functionName": "v1.events.repository.decodeNextKey",
	}

	sDec, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error decoding string %s", str)
		return nil, err
	}

	var m map[string]*dynamodb.AttributeValue
	err = json.Unmarshal(sDec, &m)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("error unmarshalling string after decoding: %s", sDec)
		return nil, err
	}

	return m, nil
}

// buildEventListModel converts the query results to a list event models
func buildEventListModels(results *dynamodb.QueryOutput) ([]*models.Event, error) {
	f := logrus.Fields{
		"functionName": "v1.events.repository.buildEventListModels",
	}
	events := make([]*models.Event, 0)

	var items []Event

	err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &items)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error unmarshalling events from database")
		return nil, err
	}

	for _, e := range items {
		events = append(events, e.toEvent())
	}

	return events, nil
}

// buildProjection builds the query projection
func buildProjection() expression.ProjectionBuilder {
	// These are the columns we want returned
	return expression.NamesList(
		expression.Name("event_id"),
		expression.Name("event_type"),
		expression.Name("event_user_id"),
		expression.Name("event_user_name"),
		expression.Name("event_lf_username"),
		expression.Name("event_project_id"),
		expression.Name("event_project_name"),
		expression.Name("event_company_id"),
		expression.Name("event_company_name"),
		expression.Name("event_time"),
		expression.Name("event_time_epoch"),
		expression.Name("event_data"),
		expression.Name("event_summary"),
		expression.Name("event_project_external_id"),
	)
}

func (repo repository) GetRecentEvents(pageSize int64) (*models.EventList, error) {
	f := logrus.Fields{
		"functionName": "v1.events.repository.GetRecentEvents",
		"pageSize":     pageSize,
	}

	ctime := time.Now()
	maxQueryDays := 30
	events := make([]*models.Event, 0)
	for queriedDays := 0; queriedDays < maxQueryDays; queriedDays++ {
		day := toDateFormat(ctime)
		eventList, err := repo.getEventByDay(day, false, pageSize)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("error fetching events by day")
			return nil, err
		}
		events = append(events, eventList...)
		if int64(len(events)) >= pageSize {
			events = events[0:pageSize]
			break
		}
		ctime = ctime.Add(-(24 * time.Hour))
	}

	return &models.EventList{
		Events: events,
	}, nil
}

func (repo repository) getEventByDay(day string, containsPII bool, pageSize int64) ([]*models.Event, error) {
	f := logrus.Fields{
		"functionName": "v1.events.repository.getEventByDay",
		"day":          day,
		"containsPII":  containsPII,
		"pageSize":     pageSize,
	}

	var condition expression.KeyConditionBuilder
	builder := expression.NewBuilder().WithProjection(buildProjection())

	indexName := "event-date-and-contains-pii-event-time-epoch-index"
	eventDateAndContainsPII := fmt.Sprintf("%s#%t", day, containsPII)
	filter := expression.Name("event_project_id").AttributeExists()
	condition = expression.Key("event_date_and_contains_pii").Equal(expression.Value(eventDateAndContainsPII))

	builder = builder.WithKeyCondition(condition).WithFilter(filter)
	// Use the nice builder to create the expression
	expr, err := builder.Build()
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error building events query")
		return nil, err
	}

	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(repo.eventsTable),
		IndexName:                 aws.String(indexName),
		Limit:                     aws.Int64(pageSize), // The maximum number of items to evaluate (not necessarily the number of matching items)
		ScanIndexForward:          aws.Bool(false),
	}

	events := make([]*models.Event, 0)

	for {
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).Warnf("error retrieving events. error = %s", errQuery.Error())
			return nil, errQuery
		}

		eventsList, modelErr := buildEventListModels(results)
		if modelErr != nil {
			log.WithFields(f).Warn("error building event models")
			return nil, modelErr
		}

		if len(eventsList) > 0 {
			events = append(events, eventsList...)
		}
		if int64(len(events)) >= pageSize {
			return events[0:pageSize], nil
		}
		if len(results.LastEvaluatedKey) != 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}
	}
	return events, nil
}

func (repo repository) AddDataToEvent(eventID, parentProjectSFID, projectSFID, projectSFName, companySFID, projectID string) error {
	f := logrus.Fields{
		"functionName":      "v1.events.repository.AddDataToEvent",
		"eventID":           eventID,
		"parentProjectSFID": parentProjectSFID,
		"projectSFID":       projectSFID,
		"projectSFName":     projectSFName,
		"companySFID":       companySFID,
		"projectID":         projectID,
	}

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(repo.eventsTable),
		Key: map[string]*dynamodb.AttributeValue{
			"event_id": {
				S: aws.String(eventID),
			},
		},
	}
	companySFIDFoundationSFID := fmt.Sprintf("%s#%s", companySFID, parentProjectSFID)
	companySFIDProjectID := fmt.Sprintf("%s#%s", companySFID, projectID)
	ue := utils.NewDynamoUpdateExpression()
	ue.AddAttributeName("#parent_project_sfid", "event_parent_project_sfid", parentProjectSFID != "")
	ue.AddAttributeName("#project_sfid", "event_project_sfid", projectSFID != "")
	ue.AddAttributeName("#project_sf_name", "event_sf_project_name", projectSFName != "")

	ue.AddAttributeName("#company_sfid", "event_company_sfid", companySFID != "")
	ue.AddAttributeName("#company_sfid_foundation_sfid", "company_sfid_foundation_sfid", companySFID != "" && parentProjectSFID != "")
	ue.AddAttributeName("#company_sfid_project_id", "company_sfid_project_id", companySFID != "" && projectID != "")

	ue.AddAttributeValue(":foundation_sfid", &dynamodb.AttributeValue{S: aws.String(parentProjectSFID)}, parentProjectSFID != "")
	ue.AddAttributeValue(":project_sfid", &dynamodb.AttributeValue{S: aws.String(projectSFID)}, projectSFID != "")
	ue.AddAttributeValue(":project_sf_name", &dynamodb.AttributeValue{S: aws.String(projectSFName)}, projectSFName != "")

	ue.AddAttributeValue(":company_sfid", &dynamodb.AttributeValue{S: aws.String(companySFID)}, companySFID != "")
	ue.AddAttributeValue(":company_sfid_foundation_sfid", &dynamodb.AttributeValue{S: aws.String(companySFIDFoundationSFID)}, companySFID != "" && parentProjectSFID != "")
	ue.AddAttributeValue(":company_sfid_project_id", &dynamodb.AttributeValue{S: aws.String(companySFIDProjectID)}, companySFID != "" && projectID != "")

	ue.AddUpdateExpression("#parent_project_sfid = :parent_project_sfid", parentProjectSFID != "")
	ue.AddUpdateExpression("#project_sfid = :project_sfid", projectSFID != "")
	ue.AddUpdateExpression("#project_sf_name = :project_sf_name", projectSFName != "")

	ue.AddUpdateExpression("#company_sfid = :company_sfid", companySFID != "")
	ue.AddUpdateExpression("#company_sfid_foundation_sfid = :company_sfid_foundation_sfid", companySFID != "" && parentProjectSFID != "")
	ue.AddUpdateExpression("#company_sfid_project_id = :company_sfid_project_id", companySFID != "" && projectID != "")
	if ue.Expression == "" {
		// nothing to update
		log.WithFields(f).Warn("not expression - nothing to update")
		return nil
	}
	input.UpdateExpression = aws.String(ue.Expression)
	input.ExpressionAttributeNames = ue.ExpressionAttributeNames
	input.ExpressionAttributeValues = ue.ExpressionAttributeValues
	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.WithFields(f).WithError(updateErr).Warnf("unable to add extra details to event : %s . error = %s", eventID, updateErr.Error())
		return updateErr
	}

	return nil
}
