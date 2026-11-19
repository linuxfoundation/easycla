// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package company

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/request"
	"github.com/aws/aws-sdk-go/service/dynamodb"
	"github.com/aws/aws-sdk-go/service/dynamodb/dynamodbiface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	v2UserServiceModels "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service/models"
)

const (
	lockTable  = "cla-test-store"
	lockDomain = "example.invalid" // the registrable domain of testCompanySite
	lockKey    = "company_create:" + lockDomain
)

// fakeLockTable is the store table as the locks use it: it applies the two conditions the locks send and refuses any other
type fakeLockTable struct {
	dynamodbiface.DynamoDBAPI
	mu    sync.Mutex
	items map[string]map[string]*dynamodb.AttributeValue
	fail  error
}

func newFakeLockTable() *fakeLockTable {
	return &fakeLockTable{items: map[string]map[string]*dynamodb.AttributeValue{}}
}

func (f *fakeLockTable) PutItemWithContext(_ aws.Context, in *dynamodb.PutItemInput, _ ...request.Option) (*dynamodb.PutItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	if aws.StringValue(in.TableName) != lockTable || aws.StringValue(in.ConditionExpression) != "attribute_not_exists(#o) OR #e < :now" ||
		aws.StringValue(in.ExpressionAttributeNames["#o"]) != createLockOwner || aws.StringValue(in.ExpressionAttributeNames["#e"]) != "expire" {
		return nil, fmt.Errorf("unexpected put: table %q condition %q", aws.StringValue(in.TableName), aws.StringValue(in.ConditionExpression))
	}
	key := aws.StringValue(in.Item["key"].S)
	if held, ok := f.items[key]; ok && held[createLockOwner] != nil {
		expire, expireErr := strconv.ParseInt(aws.StringValue(held["expire"].N), 10, 64)
		now, nowErr := strconv.ParseInt(aws.StringValue(in.ExpressionAttributeValues[":now"].N), 10, 64)
		if expireErr != nil || nowErr != nil {
			return nil, fmt.Errorf("unexpected put: expire %v now %v", expireErr, nowErr)
		}
		if expire >= now {
			return nil, awserr.New(dynamodb.ErrCodeConditionalCheckFailedException, "The conditional request failed", nil)
		}
	}
	f.items[key] = in.Item
	return &dynamodb.PutItemOutput{}, nil
}

func (f *fakeLockTable) DeleteItemWithContext(_ aws.Context, in *dynamodb.DeleteItemInput, _ ...request.Option) (*dynamodb.DeleteItemOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.fail != nil {
		return nil, f.fail
	}
	if aws.StringValue(in.TableName) != lockTable || aws.StringValue(in.ConditionExpression) != "#o = :owner" || aws.StringValue(in.ExpressionAttributeNames["#o"]) != createLockOwner {
		return nil, fmt.Errorf("unexpected delete: table %q condition %q", aws.StringValue(in.TableName), aws.StringValue(in.ConditionExpression))
	}
	key := aws.StringValue(in.Key["key"].S)
	held, ok := f.items[key]
	if !ok || held[createLockOwner] == nil || aws.StringValue(held[createLockOwner].S) != aws.StringValue(in.ExpressionAttributeValues[":owner"].S) {
		return nil, awserr.New(dynamodb.ErrCodeConditionalCheckFailedException, "The conditional request failed", nil)
	}
	delete(f.items, key)
	return &dynamodb.DeleteItemOutput{}, nil
}

func (f *fakeLockTable) keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	keys := make([]string, 0, len(f.items))
	for k := range f.items {
		keys = append(keys, k)
	}
	return keys
}

func TestDynamoCreateLocks(t *testing.T) {
	now := time.Unix(1800000000, 0)
	table := newFakeLockTable()
	locks := DynamoCreateLocks{DB: table, Table: lockTable, Now: func() time.Time { return now }}
	ctx := context.Background()

	first, err := locks.Acquire(ctx, lockDomain)
	require.NoError(t, err)
	item := table.items[lockKey]
	require.NotNil(t, item, "one item per domain under the company_create: prefix")
	assert.Equal(t, first, aws.StringValue(item[createLockOwner].S))
	assert.Equal(t, strconv.FormatInt(now.Add(6*time.Minute).Unix(), 10), aws.StringValue(item["expire"].N), "longer than the 300 second Lambda timeout")

	_, err = locks.Acquire(ctx, lockDomain)
	assert.ErrorIs(t, err, ErrCreateLockHeld)
	other, err := locks.Acquire(ctx, "other.invalid")
	require.NoError(t, err, "another domain has its own lock")
	require.NoError(t, locks.Release(ctx, "other.invalid", other))

	now = now.Add(6 * time.Minute)
	_, err = locks.Acquire(ctx, lockDomain)
	assert.ErrorIs(t, err, ErrCreateLockHeld, "held through its last second")
	now = now.Add(time.Second)
	second, err := locks.Acquire(ctx, lockDomain)
	require.NoError(t, err, "an expired lock is taken over")
	assert.NotEqual(t, first, second)

	require.NoError(t, locks.Release(ctx, lockDomain, first), "releasing a lock that was taken over is not an error")
	assert.Equal(t, []string{lockKey}, table.keys(), "and leaves the new holder's lock")
	require.NoError(t, locks.Release(ctx, lockDomain, second))
	assert.Empty(t, table.keys())

	sessionRow := map[string]*dynamodb.AttributeValue{
		"key":    {S: aws.String(lockKey)},
		"value":  {S: aws.String("{}")},
		"expire": {N: aws.String(strconv.FormatInt(now.Add(45*time.Minute).Unix(), 10))},
	}
	table.items[lockKey] = sessionRow
	third, err := locks.Acquire(ctx, lockDomain)
	require.NoError(t, err, "a row without an owner, as the legacy session store writes them, does not hold the lock")
	assert.Equal(t, third, aws.StringValue(table.items[lockKey][createLockOwner].S))
	table.items[lockKey] = sessionRow
	require.NoError(t, locks.Release(ctx, lockDomain, third), "and a lock that such a row replaced is released without an error")
	assert.Equal(t, []string{lockKey}, table.keys(), "the row is not the lock's to remove")
	delete(table.items, lockKey)

	table.fail = errors.New("dynamodb unavailable")
	_, err = locks.Acquire(ctx, lockDomain)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrCreateLockHeld, "a failed write is not a held lock")
	assert.Error(t, locks.Release(ctx, lockDomain, second))
}

// recordingLocks answers as configured and records the calls in order
type recordingLocks struct {
	mu         sync.Mutex
	acquireErr error
	releaseErr error
	calls      []string
}

func (l *recordingLocks) Acquire(_ context.Context, domain string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, "acquire "+domain)
	if l.acquireErr != nil {
		return "", l.acquireErr
	}
	return "owner-1", nil
}

func (l *recordingLocks) Release(_ context.Context, domain, owner string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.calls = append(l.calls, "release "+domain+" "+owner)
	return l.releaseErr
}

func TestCreateCompanyLock(t *testing.T) {
	withCompanyRoleFlags(t, true, true)
	previous := lfUserByEmail
	t.Cleanup(func() { lfUserByEmail = previous })
	lfUserByEmail = func(string) (*v2UserServiceModels.User, error) {
		return &v2UserServiceModels.User{Username: testCallerUsername}, nil
	}
	created := func() *memberCreateHTTP {
		return &memberCreateHTTP{memberStatus: http.StatusCreated, memberBody: `{"uid":"` + msAccountSFID + `","name":"` + testCompanyName + `"}`, scopeStatus: http.StatusCreated}
	}
	const findOrCreate = "/b2b_orgs/find-or-create"
	held := []string{"acquire " + lockDomain}
	heldAndReleased := []string{"acquire " + lockDomain, "release " + lockDomain + " owner-1"}

	t.Run("taken on the website domain and kept after the creation", func(t *testing.T) {
		locks := &recordingLocks{}
		fx := newMemberCreateFixture(t, created(), memberCreateRepo{expectRow: true})
		fx.svc.EnableCreateLock(locks)
		output, err := fx.svc.CreateCompany(context.Background(), memberCreateParams("https://www.Acme.Example.invalid/about", ""))
		require.NoError(t, err)
		require.NotNil(t, output)
		assert.Equal(t, held, locks.calls, "a created company keeps the lock until it expires")
	})

	t.Run("released after a refused creation", func(t *testing.T) {
		locks := &recordingLocks{releaseErr: errors.New("dynamodb unavailable")}
		fx := newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusConflict, memberBody: `{"message":"2 accounts"}`}, memberCreateRepo{})
		fx.svc.EnableCreateLock(locks)
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.ErrorIs(t, err, ErrCompanyAmbiguousDomain, "a failed release does not replace the creation's error")
		assert.Equal(t, heldAndReleased, locks.calls)
	})

	t.Run("kept after a find-or-create without an answer", func(t *testing.T) {
		locks := &recordingLocks{}
		fx := newMemberCreateFixture(t, &memberCreateHTTP{memberStatus: http.StatusServiceUnavailable, memberBody: msEmptyJSON}, memberCreateRepo{})
		fx.svc.EnableCreateLock(locks)
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.ErrorIs(t, err, ErrCompanyDependencyFailed)
		assert.Equal(t, held, locks.calls, "Salesforce may still be writing, so the lock stays until it expires")
	})

	t.Run("released after a rejected token request", func(t *testing.T) {
		locks := &recordingLocks{}
		fx := newMemberCreateFixture(t, &memberCreateHTTP{memberTokenStatus: http.StatusForbidden}, memberCreateRepo{})
		fx.svc.EnableCreateLock(locks)
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.ErrorIs(t, err, ErrCompanyDependencyFailed)
		assert.Zero(t, fx.transport.count(findOrCreate), "nothing was sent to member-service")
		assert.Equal(t, heldAndReleased, locks.calls)
	})

	t.Run("kept after a failed company write", func(t *testing.T) {
		locks := &recordingLocks{}
		fx := newMemberCreateFixture(t, created(), memberCreateRepo{expectRow: true, createFailure: errors.New("dynamodb timeout")})
		fx.svc.EnableCreateLock(locks)
		_, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.ErrorIs(t, err, ErrCompanyDependencyFailed)
		assert.Equal(t, held, locks.calls, "the row may have been written, so the lock stays until it expires")
	})

	t.Run("a held lock refuses the creation before any Salesforce call", func(t *testing.T) {
		locks := &recordingLocks{acquireErr: ErrCreateLockHeld}
		fx := newMemberCreateFixture(t, created(), memberCreateRepo{})
		fx.svc.EnableCreateLock(locks)
		output, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.ErrorIs(t, err, ErrCompanyCreateInProgress)
		assert.Nil(t, output)
		assert.Zero(t, fx.transport.count(findOrCreate))
		assert.Equal(t, held, locks.calls, "a lock that was not taken is not released")
	})

	t.Run("a lock that cannot be taken stops the creation before any Salesforce call", func(t *testing.T) {
		locks := &recordingLocks{acquireErr: errors.New("dynamodb unavailable")}
		fx := newMemberCreateFixture(t, created(), memberCreateRepo{})
		fx.svc.EnableCreateLock(locks)
		output, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.ErrorIs(t, err, ErrCompanyDependencyFailed)
		assert.NotErrorIs(t, err, ErrCompanyCreateInProgress)
		assert.Nil(t, output)
		assert.Zero(t, fx.transport.count(findOrCreate))
		assert.Equal(t, held, locks.calls)
	})

	t.Run("two creations at the same moment make one Salesforce call", func(t *testing.T) {
		table := newFakeLockTable()
		transport := created()
		arrived, proceed := make(chan struct{}), make(chan struct{})
		var gated atomic.Bool
		transport.memberGate = func() {
			if gated.CompareAndSwap(false, true) {
				close(arrived)
				<-proceed
			}
		}
		fx := newMemberCreateFixture(t, transport, memberCreateRepo{expectRow: true})
		fx.svc.EnableCreateLock(DynamoCreateLocks{DB: table, Table: lockTable})
		type result struct {
			output *models.CompanyOutput
			err    error
		}
		winner := make(chan result, 1)
		go func() {
			output, err := fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
			winner <- result{output, err}
		}()
		select {
		case <-arrived:
		case <-time.After(10 * time.Second):
			t.Fatal("the first creation did not reach member-service")
		}
		output, err := fx.svc.CreateCompany(context.Background(), memberCreateParams("http://www.acme.example.invalid", ""))
		assert.ErrorIs(t, err, ErrCompanyCreateInProgress, "the second request for the domain is refused while the first runs")
		assert.Nil(t, output)
		assert.Equal(t, 1, transport.count(findOrCreate), "no second Salesforce account can be created")
		close(proceed)
		select {
		case got := <-winner:
			require.NoError(t, got.err)
			require.NotNil(t, got.output)
			assert.Equal(t, msAccountSFID, got.output.CompanyID)
		case <-time.After(10 * time.Second):
			t.Fatal("the first creation did not finish")
		}
		// the fixture's index lookups still answer "not found": a request right after the creation would pass them
		output, err = fx.svc.CreateCompany(context.Background(), memberCreateParams(testCompanySite, ""))
		assert.ErrorIs(t, err, ErrCompanyCreateInProgress, "refused until the lease ends, while the indexes may not list the new company")
		assert.Nil(t, output)
		assert.Equal(t, 1, transport.count(findOrCreate))
		assert.Equal(t, []string{lockKey}, table.keys())
	})
}
