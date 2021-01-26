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

	"github.com/communitybridge/easycla/cla-backend-go/gen/models"
	eventOps "github.com/communitybridge/easycla/cla-backend-go/gen/restapi/operations/events"
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
)

// constants
const (
	HugePageSize    = 10000
	DefaultPageSize = 10
)

// Repository interface defines methods of event repository service
type Repository interface {
	CreateEvent(event *models.Event) error
	AddDataToEvent(eventID, foundationSFID, projectSFID, projectSFName, companySFID, projectID string) error
	SearchEvents(params *eventOps.SearchEventsParams, pageSize int64) (*models.EventList, error)
	GetRecentEvents(pageSize int64) (*models.EventList, error)

	GetCompanyFoundationEvents(companySFID, companyID, foundationSFID string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error)
	GetCompanyClaGroupEvents(companySFID, companyID, claGroupID string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error)
	GetCompanyEvents(companyID, eventType string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error)
	GetFoundationEvents(foundationSFID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error)
	GetClaGroupEvents(claGroupID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error)
}

// repository data model
type repository struct {
	stage          string
	dynamoDBClient *dynamodb.DynamoDB
}

// NewRepository creates a new instance of the event repository
func NewRepository(awsSession *session.Session, stage string) Repository {
	return &repository{
		stage:          stage,
		dynamoDBClient: dynamodb.New(awsSession),
	}
}

func toDateFormat(t time.Time) string {
	//DD-MM-YYYY format
	return t.Format("02-01-2006")
}

// Create event will create event in database.
func (repo *repository) CreateEvent(event *models.Event) error {
	if event.UserID == "" {
		return ErrUserIDRequired
	}
	if event.EventType == "" {
		return ErrEventTypeRequired
	}
	eventID, err := uuid.NewV4()
	if err != nil {
		log.Warnf("Unable to generate a UUID for a whitelist request, error: %v", err)
		return err
	}

	currentTime, currentTimeString := utils.CurrentTime()
	input := &dynamodb.PutItemInput{
		Item:      map[string]*dynamodb.AttributeValue{},
		TableName: aws.String(fmt.Sprintf("cla-%s-events", repo.stage)),
	}
	eventDateAndContainsPII := fmt.Sprintf("%s#%t", toDateFormat(currentTime), event.ContainsPII)
	addAttribute(input.Item, "event_id", eventID.String())
	addAttribute(input.Item, "event_type", event.EventType)
	addAttribute(input.Item, "event_user_id", event.UserID)
	addAttribute(input.Item, "event_user_name", event.UserName)
	addAttribute(input.Item, "event_lf_username", event.LfUsername)
	addAttribute(input.Item, "event_user_name_lower", strings.ToLower(event.UserName))
	addAttribute(input.Item, "event_time", currentTimeString)
	addAttribute(input.Item, "event_data", event.EventData)
	addAttribute(input.Item, "event_summary", event.EventSummary)
	addAttribute(input.Item, "event_company_id", event.EventCompanyID)
	addAttribute(input.Item, "event_company_name", event.EventCompanyName)
	addAttribute(input.Item, "event_company_name_lower", strings.ToLower(event.EventCompanyName))
	addAttribute(input.Item, "event_project_id", event.EventProjectID)
	addAttribute(input.Item, "event_project_name", event.EventProjectName)
	addAttribute(input.Item, "event_project_name_lower", strings.ToLower(event.EventProjectName))
	addAttribute(input.Item, "event_date", toDateFormat(currentTime))
	addAttribute(input.Item, "event_project_external_id", event.EventProjectExternalID)
	addAttribute(input.Item, "event_date_and_contains_pii", eventDateAndContainsPII)
	input.Item["contains_pii"] = &dynamodb.AttributeValue{BOOL: &event.ContainsPII}
	input.Item["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(currentTime.Unix(), 10))}
	if event.EventCompanyID != "" && event.EventProjectExternalID != "" {
		companyIDexternalProjectID := fmt.Sprintf("%s#%s", event.EventCompanyID, event.EventProjectExternalID)
		addAttribute(input.Item, "company_id_external_project_id", companyIDexternalProjectID)
	}

	_, err = repo.dynamoDBClient.PutItem(input)
	if err != nil {
		log.Warnf("Unable to create a new event, error: %v", err)
		return err
	}
	log.Printf("added event : %s", eventID.String())

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
		filterExpression := expression.Name("event_data").Contains(*params.SearchTerm)
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
	if params.ProjectID == nil {
		return nil, errors.New("invalid request. projectID is compulsory")
	}
	var condition expression.KeyConditionBuilder
	var indexName, pk, sk string
	builder := expression.NewBuilder().WithProjection(buildProjection())
	// The table we're interested in
	tableName := fmt.Sprintf("cla-%s-events", repo.stage)

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
		TableName:                 aws.String(tableName),
		IndexName:                 aws.String(indexName),
		Limit:                     aws.Int64(pageSize), // The maximum number of items to evaluate (not necessarily the number of matching items)
	}
	if params.SortOrder != nil && *params.SortOrder == "desc" {
		queryInput.ScanIndexForward = aws.Bool(false)
	}

	if params.NextKey != nil {
		log.Debugf("Received a nextKey, value: %s", *params.NextKey)
		queryInput.ExclusiveStartKey, err = fromString(*params.NextKey)
		if err != nil {
			return nil, err
		}
	}

	var lastEvaluatedKey string
	events := make([]*models.Event, 0)

	for ok := true; ok; ok = lastEvaluatedKey != "" {
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.Warnf("error retrieving events. error = %s", errQuery.Error())
			return nil, errQuery
		}

		eventsList, modelErr := buildEventListModels(results)
		if modelErr != nil {
			return nil, modelErr
		}

		events = append(events, eventsList...)
		if len(results.LastEvaluatedKey) != 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		}
		lastEvaluatedKey, err = toString(results.LastEvaluatedKey)
		if err != nil {
			return nil, err
		}
		if int64(len(events)) >= pageSize {
			break
		}
	}

	return &models.EventList{
		Events:  events,
		NextKey: lastEvaluatedKey,
	}, nil
}

// queryEventsTable queries events table on index
func (repo *repository) queryEventsTable(indexName string, condition expression.KeyConditionBuilder, filter *expression.ConditionBuilder, nextKey *string, pageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	f := logrus.Fields{
		"functionName": "events.queryEventsTable",
		"indexName":    indexName,
		"nextKey":      aws.StringValue(nextKey),
		"pageSize":     aws.Int64Value(pageSize),
		"all":          all,
		"searchTerm":   aws.StringValue(searchTerm),
	}

	log.WithFields(f).Debug("querying events table")
	builder := expression.NewBuilder() // .WithProjection(buildProjection())

	// The table we're interested in
	tableName := fmt.Sprintf("cla-%s-events", repo.stage)

	builder = builder.WithKeyCondition(condition)
	if filter != nil {
		builder = builder.WithFilter(*filter)
	}
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
		TableName:                 aws.String(tableName),
		IndexName:                 aws.String(indexName),
		ScanIndexForward:          aws.Bool(false),
	}

	if all {
		pageSize = aws.Int64(HugePageSize)
	} else {
		if pageSize == nil {
			pageSize = aws.Int64(DefaultPageSize)
		}
	}

	if searchTerm != nil {
		// since we are filtering data in client side, we should use large pageSize to avoid recursive query
		queryInput.Limit = aws.Int64(HugePageSize)
	} else {
		queryInput.Limit = aws.Int64(*pageSize + 1)
	}

	if nextKey != nil && !all {
		// log.Debugf("Received a nextKey, value: %s", *nextKey)
		queryInput.ExclusiveStartKey, err = fromString(*nextKey)
		if err != nil {
			return nil, err
		}
	}

	// log.WithField("queryInput", *queryInput).Debug("query")
	var lastEvaluatedKey string
	events := make([]*models.Event, 0)

	if searchTerm != nil {
		searchTerm = aws.String(strings.ToLower(*searchTerm))
	}

	for ok := true; ok; ok = lastEvaluatedKey != "" {
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.WithFields(f).WithError(errQuery).Warnf("error retrieving events. error = %s", errQuery.Error())
			return nil, errQuery
		}

		eventsList, modelErr := buildEventListModels(results)
		if modelErr != nil {
			return nil, modelErr
		}
		if searchTerm != nil {
			for _, event := range eventsList {
				if !all {
					if int64(len(events)) >= (*pageSize + 1) {
						break
					}
				}
				if strings.Contains(strings.ToLower(event.EventData), *searchTerm) {
					events = append(events, event)
				}
			}
		} else {
			events = append(events, eventsList...)
		}

		if len(results.LastEvaluatedKey) != 0 {
			queryInput.ExclusiveStartKey = results.LastEvaluatedKey
		} else {
			break
		}

		if !all {
			if int64(len(events)) >= (*pageSize + 1) {
				break
			}
		}
	}
	if !all {
		if int64(len(events)) > *pageSize {
			events = events[0:*pageSize]
			lastEvaluatedKey, err = buildNextKey(indexName, events[*pageSize-1])
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("unable to build nextKey. index = %s, event = %#v error = %s", indexName, events[*pageSize-1], err.Error())
			}
		} else {
			events = events[0:int64(len(events))]
		}
	}

	if len(events) > 0 {
		return &models.EventList{
			Events:  events,
			NextKey: lastEvaluatedKey,
		}, nil
	}

	// Just return an empty response - no events - just an empty list, and no nextKey
	return &models.EventList{
		Events: []*models.Event{},
	}, nil
}

func buildNextKey(indexName string, event *models.Event) (string, error) {
	nextKey := make(map[string]*dynamodb.AttributeValue)
	nextKey["event_id"] = &dynamodb.AttributeValue{S: aws.String(event.EventID)}
	switch indexName {
	case CompanySFIDFoundationSFIDEpochIndex:
		nextKey["company_sfid_foundation_sfid"] = &dynamodb.AttributeValue{
			S: aws.String(fmt.Sprintf("%s#%s", event.EventCompanySFID, event.EventFoundationSFID)),
		}
		nextKey["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(event.EventTimeEpoch, 10))}
	case CompanySFIDProjectIDEpochIndex:
		nextKey["company_sfid_project_id"] = &dynamodb.AttributeValue{
			S: aws.String(fmt.Sprintf("%s#%s", event.EventCompanySFID, event.EventProjectID)),
		}
		nextKey["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(event.EventTimeEpoch, 10))}
	case EventFoundationSFIDEpochIndex:
		nextKey["event_foundation_sfid"] = &dynamodb.AttributeValue{S: aws.String(event.EventFoundationSFID)}
		nextKey["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(event.EventTimeEpoch, 10))}
	case EventProjectIDEpochIndex:
		nextKey["event_project_id"] = &dynamodb.AttributeValue{S: aws.String(event.EventProjectID)}
		nextKey["event_time_epoch"] = &dynamodb.AttributeValue{N: aws.String(strconv.FormatInt(event.EventTimeEpoch, 10))}
	case CompanyIDEventTypeIndex:
		nextKey["company_id"] = &dynamodb.AttributeValue{S: aws.String(event.EventCompanyID)}
		nextKey["event_type"] = &dynamodb.AttributeValue{S: aws.String(event.EventType)}
	}
	return toString(nextKey)
}

// GetCompanyFoundationEvents returns the list of events for foundation and company
func (repo *repository) GetCompanyFoundationEvents(companySFID, companyID, foundationSFID string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error) {
	key := fmt.Sprintf("%s#%s", companySFID, foundationSFID)
	keyCondition := expression.Key("company_sfid_foundation_sfid").Equal(expression.Value(key))
	var filter expression.ConditionBuilder
	if companyID != "" {
		filter = expression.Name("company_id").Equal(expression.Value(companyID))
	}
	return repo.queryEventsTable(CompanySFIDFoundationSFIDEpochIndex, keyCondition, &filter, nextKey, paramPageSize, all, nil)
}

// GetCompanyClaGroupEvents returns the list of events for cla group and the company
func (repo *repository) GetCompanyClaGroupEvents(companySFID, companyID, claGroupID string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error) {
	key := fmt.Sprintf("%s#%s", companySFID, claGroupID)
	keyCondition := expression.Key("company_sfid_project_id").Equal(expression.Value(key))
	var filter expression.ConditionBuilder
	if companyID != "" {
		filter = expression.Name("company_id").Equal(expression.Value(companyID))
	}
	return repo.queryEventsTable(CompanySFIDProjectIDEpochIndex, keyCondition, &filter, nextKey, paramPageSize, all, nil)
}

// GetCompanyEvents returns the list of events for given company id and event types
func (repo *repository) GetCompanyEvents(companyID, eventType string, nextKey *string, paramPageSize *int64, all bool) (*models.EventList, error) {
	keyCondition := expression.Key("company_id").Equal(expression.Value(companyID)).And(
		expression.Key("event_type").Equal(expression.Value(eventType)))

	return repo.queryEventsTable(CompanyIDEventTypeIndex, keyCondition, nil, nextKey, paramPageSize, all, nil)
}

// GetFoundationEvents returns the list of foundation events
func (repo *repository) GetFoundationEvents(foundationSFID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	keyCondition := expression.Key("event_foundation_sfid").Equal(expression.Value(foundationSFID))
	return repo.queryEventsTable(EventFoundationSFIDEpochIndex, keyCondition, nil, nextKey, paramPageSize, all, searchTerm)
}

// GetClaGroupEvents returns the list of cla-group events
func (repo *repository) GetClaGroupEvents(claGroupID string, nextKey *string, paramPageSize *int64, all bool, searchTerm *string) (*models.EventList, error) {
	keyCondition := expression.Key("event_project_id").Equal(expression.Value(claGroupID))
	return repo.queryEventsTable(EventProjectIDEpochIndex, keyCondition, nil, nextKey, paramPageSize, all, searchTerm)
}

// toString encodes the map as a string
func toString(in map[string]*dynamodb.AttributeValue) (string, error) {
	if len(in) == 0 {
		return "", nil
	}
	b, err := json.Marshal(in)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(b), nil
}

// fromString converts the string to a map
func fromString(str string) (map[string]*dynamodb.AttributeValue, error) {
	sDec, err := base64.StdEncoding.DecodeString(str)
	if err != nil {
		return nil, err
	}
	var m map[string]*dynamodb.AttributeValue
	err = json.Unmarshal(sDec, &m)
	if err != nil {
		return nil, err
	}
	return m, nil
}

// buildEventListModel converts the query results to a list event models
func buildEventListModels(results *dynamodb.QueryOutput) ([]*models.Event, error) {
	events := make([]*models.Event, 0)

	var items []Event

	err := dynamodbattribute.UnmarshalListOfMaps(results.Items, &items)
	if err != nil {
		log.Warnf("error unmarshalling events from database, error: %v",
			err)
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
	ctime := time.Now()
	maxQueryDays := 30
	events := make([]*models.Event, 0)
	for queriedDays := 0; queriedDays < maxQueryDays; queriedDays++ {
		day := toDateFormat(ctime)
		eventList, err := repo.getEventByDay(day, false, pageSize)
		if err != nil {
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
	tableName := fmt.Sprintf("cla-%s-events", repo.stage)
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
		return nil, err
	}
	// Assemble the query input parameters
	queryInput := &dynamodb.QueryInput{
		ExpressionAttributeNames:  expr.Names(),
		ExpressionAttributeValues: expr.Values(),
		KeyConditionExpression:    expr.KeyCondition(),
		ProjectionExpression:      expr.Projection(),
		FilterExpression:          expr.Filter(),
		TableName:                 aws.String(tableName),
		IndexName:                 aws.String(indexName),
		Limit:                     aws.Int64(pageSize), // The maximum number of items to evaluate (not necessarily the number of matching items)
		ScanIndexForward:          aws.Bool(false),
	}

	events := make([]*models.Event, 0)

	for {
		results, errQuery := repo.dynamoDBClient.Query(queryInput)
		if errQuery != nil {
			log.Warnf("error retrieving events. error = %s", errQuery.Error())
			return nil, errQuery
		}

		eventsList, modelErr := buildEventListModels(results)
		if modelErr != nil {
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

func (repo repository) AddDataToEvent(eventID, foundationSFID, projectSFID, projectSFName, companySFID, projectID string) error {
	tableName := fmt.Sprintf("cla-%s-events", repo.stage)
	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]*dynamodb.AttributeValue{
			"event_id": {
				S: aws.String(eventID),
			},
		},
	}
	companySFIDFoundationSFID := fmt.Sprintf("%s#%s", companySFID, foundationSFID)
	companySFIDProjectID := fmt.Sprintf("%s#%s", companySFID, projectID)
	ue := utils.NewDynamoUpdateExpression()
	ue.AddAttributeName("#foundation_sfid", "event_foundation_sfid", foundationSFID != "")
	ue.AddAttributeName("#project_sfid", "event_project_sfid", projectSFID != "")
	ue.AddAttributeName("#project_sf_name", "event_sf_project_name", projectSFName != "")
	ue.AddAttributeName("#company_sfid", "event_company_sfid", companySFID != "")
	ue.AddAttributeName("#company_sfid_foundation_sfid", "company_sfid_foundation_sfid", companySFID != "" && foundationSFID != "")
	ue.AddAttributeName("#company_sfid_project_id", "company_sfid_project_id", companySFID != "" && projectID != "")

	ue.AddAttributeValue(":foundation_sfid", &dynamodb.AttributeValue{S: aws.String(foundationSFID)}, foundationSFID != "")
	ue.AddAttributeValue(":project_sfid", &dynamodb.AttributeValue{S: aws.String(projectSFID)}, projectSFID != "")
	ue.AddAttributeValue(":project_sf_name", &dynamodb.AttributeValue{S: aws.String(projectSFName)}, projectSFName != "")
	ue.AddAttributeValue(":company_sfid", &dynamodb.AttributeValue{S: aws.String(companySFID)}, companySFID != "")
	ue.AddAttributeValue(":company_sfid_foundation_sfid", &dynamodb.AttributeValue{S: aws.String(companySFIDFoundationSFID)}, companySFID != "" && foundationSFID != "")
	ue.AddAttributeValue(":company_sfid_project_id", &dynamodb.AttributeValue{S: aws.String(companySFIDProjectID)}, companySFID != "" && projectID != "")

	ue.AddUpdateExpression("#foundation_sfid = :foundation_sfid", foundationSFID != "")
	ue.AddUpdateExpression("#project_sfid = :project_sfid", projectSFID != "")
	ue.AddUpdateExpression("#project_sf_name = :project_sf_name", projectSFName != "")
	ue.AddUpdateExpression("#company_sfid = :company_sfid", companySFID != "")
	ue.AddUpdateExpression("#company_sfid_foundation_sfid = :company_sfid_foundation_sfid", companySFID != "" && foundationSFID != "")
	ue.AddUpdateExpression("#company_sfid_project_id = :company_sfid_project_id", companySFID != "" && projectID != "")
	if ue.Expression == "" {
		// nothing to update
		return nil
	}
	input.UpdateExpression = aws.String(ue.Expression)
	input.ExpressionAttributeNames = ue.ExpressionAttributeNames
	input.ExpressionAttributeValues = ue.ExpressionAttributeValues
	_, updateErr := repo.dynamoDBClient.UpdateItem(input)
	if updateErr != nil {
		log.Debugf("update input: %v", input)
		log.Warnf("unable to add extra details to event : %s . error = %s", eventID, updateErr.Error())
		return updateErr
	}
	return nil
}
