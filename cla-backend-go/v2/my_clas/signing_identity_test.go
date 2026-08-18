// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"errors"
	"strings"
	"testing"

	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	callerLFUsername     = "contributor"
	chosenGithubID       = int64(87654321)
	chosenGithubUsername = "octocat-work"
	otherGithubID        = int64(12345678)
)

// bind submits the chosen account the way the calling service does - as a plain value this
// side does not verify. Every test goes through it so that no test can accidentally assert
// on an ownership check this service no longer performs.
func bind(svc *service, ctx context.Context) (*models.SigningIdentity, error) {
	return svc.BindSigningIdentity(ctx, chosenGithubID, chosenGithubUsername)
}

// fakeUsersWriter records what was written so tests can assert on the record rather than
// only on the returned error. Several of this feature's requirements are about a record
// being left alone, and an assertion on the error alone passes for an implementation that
// returns an error after having already written.
type fakeUsersWriter struct {
	byID      map[string]*v1Models.User
	created   []*v1Models.User
	updates   map[string]map[string]interface{}
	createErr error
	updateErr error
	getErr    error
	// updateReadBackMisses reproduces the users service's own convention: it applies the
	// update, then re-reads the record without consistency and returns no record and no
	// error when that read misses.
	updateReadBackMisses bool
}

func newFakeUsersWriter(records ...*v1Models.User) *fakeUsersWriter {
	byID := map[string]*v1Models.User{}
	for _, record := range records {
		byID[record.UserID] = record
	}
	return &fakeUsersWriter{byID: byID, updates: map[string]map[string]interface{}{}}
}

func (f *fakeUsersWriter) CreateUser(newUser *v1Models.User, _ *user.CLAUser) (*v1Models.User, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	created := *newUser
	created.UserID = "created-user-id"
	f.created = append(f.created, &created)
	f.byID[created.UserID] = &created
	return &created, nil
}

func (f *fakeUsersWriter) UpdateUser(userID string, updates map[string]interface{}) (*v1Models.User, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.updates[userID] = updates

	record, ok := f.byID[userID]
	if !ok {
		return nil, errors.New("no such user record")
	}
	updated := *record
	if githubID, isInt64 := updates["user_github_id"].(int64); isInt64 {
		updated.GithubID = formatGithubID(githubID)
	}
	if handle, ok := updates["user_github_username"].(string); ok {
		updated.GithubUsername = handle
	}
	if lfUsername, ok := updates["lf_username"].(string); ok {
		updated.LfUsername = lfUsername
	}
	f.byID[userID] = &updated
	if f.updateReadBackMisses {
		return nil, nil
	}
	return &updated, nil
}

func (f *fakeUsersWriter) GetUser(userID string) (*v1Models.User, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return f.byID[userID], nil
}

// newBindingService wires the post-write confirmation to the writer's own store, so a test
// asserting on the record and the confirmation the service performs are reading the same
// thing. Production gets the same property from a strongly consistent base-table read.
func newBindingService(repo Repository, writer UsersWriter) *service {
	if fake, isFake := repo.(*fakeRepo); isFake && fake.consistentRead == nil {
		if store, hasStore := writer.(*fakeUsersWriter); hasStore {
			fake.consistentRead = func(userID string) (*v1Models.User, error) {
				return store.GetUser(userID)
			}
		}
	}
	return &service{repo: repo, usersWriter: writer}
}

// callerContext builds the request context the authentication middleware would produce for
// an authenticated contributor. It carries an LF identity and nothing about GitHub: which
// account the contributor owns is not something this service is told or checks.
func callerContext() context.Context {
	return user.ContextWithVerifiedCaller(context.Background(), &user.CLAUser{
		LFUsername: callerLFUsername,
		LFEmail:    "contributor@example.org",
		Name:       "A Contributor",
	})
}

func assertRefused(t *testing.T, err error, reason string) {
	t.Helper()
	var refusal *Refusal
	require.True(t, errors.As(err, &refusal), "expected a refusal, got %v", err)
	assert.Equal(t, reason, refusal.Reason)
}

// --- Refusals (T015) ---------------------------------------------------------------

func TestBindSigningIdentity_NoLFIdentityRefusesAndWritesNothing(t *testing.T) {
	writer := newFakeUsersWriter()
	svc := newBindingService(&fakeRepo{}, writer)

	// No verified caller on the context at all - the middleware never ran or the token
	// was unverifiable. The LF identity is the only verified thing this write rests on,
	// so its absence has to stop the write rather than fall through to a create.
	_, err := bind(svc, context.Background())

	assertRefused(t, err, ReasonIdentityUnavailable)
	assert.Empty(t, writer.created, "a refusal must write nothing")
	assert.Empty(t, writer.updates, "a refusal must write nothing")
}

// Documents the trust boundary as a test rather than only as a comment, so that anyone
// re-adding an ownership check here has to delete an assertion that says not to. The
// account submitted is recorded whether or not this service could show it is the caller's,
// because establishing that is the calling service's job.
func TestBindSigningIdentity_SubmittedAccountIsNotCheckedForOwnership(t *testing.T) {
	writer := newFakeUsersWriter()
	svc := newBindingService(&fakeRepo{}, writer)

	result, err := bind(svc, callerContext())

	require.NoError(t, err)
	assert.Equal(t, chosenGithubID, *result.GithubID)
	require.Len(t, writer.created, 1)
	assert.Equal(t, formatGithubID(chosenGithubID), writer.created[0].GithubID)
}

func TestBindSigningIdentity_RecordedMismatchRefuses(t *testing.T) {
	// The record the resolution settles on carries a different account than the one
	// submitted. This is the one check the caller cannot make for itself, because only
	// this side sees what the store accepted.
	record := &v1Models.User{UserID: "user-1", LfUsername: callerLFUsername, GithubID: formatGithubID(chosenGithubID)}
	repo := &fakeRepo{byGithubID: map[int64][]*v1Models.User{chosenGithubID: {record}}}

	writer := newFakeUsersWriter(&v1Models.User{
		UserID:     "user-1",
		LfUsername: callerLFUsername,
		GithubID:   formatGithubID(otherGithubID),
	})
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	assertRefused(t, err, ReasonRecordedMismatch)
}

// --- Resolution paths (T016) -------------------------------------------------------

func TestBindSigningIdentity_ExistingRecordIsReusedNotDuplicated(t *testing.T) {
	record := &v1Models.User{
		UserID:         "user-1",
		LfUsername:     callerLFUsername,
		GithubID:       formatGithubID(chosenGithubID),
		GithubUsername: "octocat-work",
	}
	repo := &fakeRepo{byGithubID: map[int64][]*v1Models.User{chosenGithubID: {record}}}
	writer := newFakeUsersWriter(record)
	svc := newBindingService(repo, writer)

	result, err := bind(svc, callerContext())

	require.NoError(t, err)
	assert.Equal(t, "user-1", *result.UserID)
	assert.Equal(t, chosenGithubID, *result.GithubID)
	assert.Equal(t, outcomeMatched, result.Outcome)
	assert.Empty(t, writer.created, "an existing record must not be duplicated")
	assert.Empty(t, writer.updates, "a matched record needs no write")
}

func TestBindSigningIdentity_FirstTimeSignerGetsARecordCarryingBoth(t *testing.T) {
	writer := newFakeUsersWriter()
	svc := newBindingService(&fakeRepo{}, writer)

	result, err := bind(svc, callerContext())

	require.NoError(t, err)
	assert.Equal(t, outcomeCreated, result.Outcome)
	require.Len(t, writer.created, 1)
	assert.Equal(t, callerLFUsername, writer.created[0].LfUsername)
	assert.Equal(t, formatGithubID(chosenGithubID), writer.created[0].GithubID)
	assert.Equal(t, "octocat-work", writer.created[0].GithubUsername)
}

func TestBindSigningIdentity_ContestedRecordRefusesAndIsLeftUntouched(t *testing.T) {
	contested := &v1Models.User{
		UserID:     "user-someone-else",
		LfUsername: "someone-else",
		GithubID:   formatGithubID(chosenGithubID),
	}
	repo := &fakeRepo{byGithubID: map[int64][]*v1Models.User{chosenGithubID: {contested}}}
	writer := newFakeUsersWriter(contested)
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	assertRefused(t, err, ReasonRecordConflict)
	// Asserted on the record, not merely on the error: an implementation that reassigns
	// and then errors would pass an error-only assertion.
	assert.Equal(t, "someone-else", writer.byID["user-someone-else"].LfUsername)
	assert.Empty(t, writer.updates)
}

func TestBindSigningIdentity_OwnRecordInDifferentCaseIsNotContested(t *testing.T) {
	// Same contributor, recorded with different casing to the one their token claims. The
	// handler already treats the two identities on a request as equal regardless of case, so
	// treating them as different people here would refuse someone their own record.
	own := &v1Models.User{
		UserID:     resolvedUserID,
		LfUsername: strings.ToUpper(callerLFUsername),
		GithubID:   formatGithubID(chosenGithubID),
	}
	repo := &fakeRepo{byGithubID: map[int64][]*v1Models.User{chosenGithubID: {own}}}
	writer := newFakeUsersWriter(own)
	svc := newBindingService(repo, writer)

	identity, err := bind(svc, callerContext())

	require.NoError(t, err)
	assert.Equal(t, resolvedUserID, *identity.UserID)
	assert.Equal(t, outcomeMatched, identity.Outcome)
}

func TestBindSigningIdentity_SeveralRecordsHoldingTheAccountRefuse(t *testing.T) {
	repo := &fakeRepo{byGithubID: map[int64][]*v1Models.User{
		chosenGithubID: {
			{UserID: "user-1", LfUsername: callerLFUsername, GithubID: formatGithubID(chosenGithubID)},
			{UserID: "user-2", LfUsername: "someone-else", GithubID: formatGithubID(chosenGithubID)},
		},
	}}
	writer := newFakeUsersWriter()
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	// The index does not enforce uniqueness, so resolving to whichever row sorts first
	// would attach the signature to an arbitrary one of two contested records.
	assertRefused(t, err, ReasonDuplicateGithubID)
	assert.Empty(t, writer.updates)
}

// A record that resolves and holds the right account but carries no identifier passes every
// other check and still dead-ends: the Console renders "invalid user ID in the URL", which the
// contributor reads as a broken product rather than a failed lookup.
func TestBindSigningIdentity_EmptyIdentifierIsNeverHandedOff(t *testing.T) {
	anonymous := &v1Models.User{UserID: "", LfUsername: callerLFUsername, GithubID: formatGithubID(chosenGithubID)}
	repo := &fakeRepo{byGithubID: map[int64][]*v1Models.User{chosenGithubID: {anonymous}}}
	writer := newFakeUsersWriter(anonymous)
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	require.Error(t, err)
	var refusal *Refusal
	require.True(t, errors.As(err, &refusal))
}

// --- Unclaimed records -------------------------------------------------------------

// A record created by a GitHub-only flow: it holds the account but names nobody, so the
// CLA never appears in anyone's list. Writing the caller's LF identity onto it would
// reunite the rightful contributor with their signature history - or hand it to whoever
// asked first, since the account number arrives as the caller's own submission and nothing
// here can tell those two cases apart.
func TestBindSigningIdentity_UnclaimedRecordIsRefusedAndLeftUntouched(t *testing.T) {
	detached := &v1Models.User{UserID: "user-detached", GithubID: formatGithubID(chosenGithubID)}
	repo := &fakeRepo{byGithubID: map[int64][]*v1Models.User{chosenGithubID: {detached}}}
	writer := newFakeUsersWriter(detached)
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	assertRefused(t, err, ReasonRecordUnclaimed)
	// Asserted on the record, not only on the error: an implementation that claims the
	// record and then errors would pass an error-only assertion, and the signatures on
	// that record would already belong to somebody else.
	assert.Empty(t, writer.updates)
	assert.Equal(t, "", writer.byID["user-detached"].LfUsername)
}

// The refusal must not be worked around by falling through to the caller's own record:
// that would leave the account on two rows, which is the state this resolution order
// exists to prevent.
func TestBindSigningIdentity_UnclaimedRecordDoesNotFallThroughToTheCallersRecord(t *testing.T) {
	detached := &v1Models.User{UserID: "user-detached", GithubID: formatGithubID(chosenGithubID)}
	repo := &fakeRepo{
		byGithubID: map[int64][]*v1Models.User{chosenGithubID: {detached}},
		// The middleware creates an LF-only record on every request whose lookups miss,
		// so the caller plausibly has one by the time this runs.
		byLFUsername: map[string][]*v1Models.User{callerLFUsername: {{UserID: "user-lf-only", LfUsername: callerLFUsername}}},
	}
	writer := newFakeUsersWriter(detached)
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	assertRefused(t, err, ReasonRecordUnclaimed)
	assert.Empty(t, writer.created)
	assert.NotContains(t, writer.updates, "user-lf-only")
}

// The case most likely to be missed, and the most damaging to get wrong: the obvious
// "no GitHub record, so adopt the caller's record" shortcut walks straight into it.
func TestBindSigningIdentity_CallersRecordHoldingAnotherAccountIsNeverOverwritten(t *testing.T) {
	own := &v1Models.User{
		UserID:         "user-own",
		LfUsername:     callerLFUsername,
		GithubID:       formatGithubID(otherGithubID),
		GithubUsername: "octocat",
	}
	repo := &fakeRepo{byLFUsername: map[string][]*v1Models.User{callerLFUsername: {own}}}
	writer := newFakeUsersWriter(own)
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	assertRefused(t, err, ReasonLFRecordAlreadyBound)
	// Asserted on the record: an overwrite here is invisible from the UI and silently
	// stops the first account's commits matching anything.
	assert.Equal(t, formatGithubID(otherGithubID), writer.byID["user-own"].GithubID)
	assert.Empty(t, writer.updates)
}

func TestBindSigningIdentity_CallersRecordWithNoAccountAdoptsIt(t *testing.T) {
	own := &v1Models.User{UserID: "user-own", LfUsername: callerLFUsername}
	repo := &fakeRepo{byLFUsername: map[string][]*v1Models.User{callerLFUsername: {own}}}
	writer := newFakeUsersWriter(own)
	svc := newBindingService(repo, writer)

	result, err := bind(svc, callerContext())

	require.NoError(t, err)
	assert.Equal(t, outcomeAdopted, result.Outcome)
	assert.Equal(t, "user-own", *result.UserID)
	assert.Empty(t, writer.created, "the caller's existing record must be used, not duplicated")
}

// The account number must reach the update path as an integer. A decimal string would be
// stored as a DynamoDB string attribute, which the numeric key condition on the
// account-number index would never match again - the association would look right in the
// record and be invisible to every lookup.
func TestBindSigningIdentity_AdoptWritesTheAccountNumberAsAnInteger(t *testing.T) {
	own := &v1Models.User{UserID: "user-own", LfUsername: callerLFUsername}
	repo := &fakeRepo{byLFUsername: map[string][]*v1Models.User{callerLFUsername: {own}}}
	writer := newFakeUsersWriter(own)
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	require.NoError(t, err)
	require.Contains(t, writer.updates, "user-own")
	assert.IsType(t, int64(0), writer.updates["user-own"]["user_github_id"])
	assert.Equal(t, chosenGithubID, writer.updates["user-own"]["user_github_id"])
}

// A request may carry no handle at all - it is optional. The handle keys its own index, so
// an empty string is rejected as an index key and takes the whole write down with it, which
// would cost the contributor the account number too.
func TestBindSigningIdentity_AdoptWithNoHandleWritesNoHandleAtAll(t *testing.T) {
	own := &v1Models.User{UserID: "user-own", LfUsername: callerLFUsername}
	repo := &fakeRepo{byLFUsername: map[string][]*v1Models.User{callerLFUsername: {own}}}
	writer := newFakeUsersWriter(own)
	svc := newBindingService(repo, writer)

	// Submitted directly rather than through bind: the blank handle is the subject.
	result, err := svc.BindSigningIdentity(callerContext(), chosenGithubID, "")

	require.NoError(t, err)
	assert.Equal(t, outcomeAdopted, result.Outcome)
	require.Contains(t, writer.updates, "user-own")
	assert.Equal(t, chosenGithubID, writer.updates["user-own"]["user_github_id"])
	assert.NotContains(t, writer.updates["user-own"], "user_github_username",
		"a blank handle is absent, not a value to record")
}

// The write succeeded; only the writer's own eventually consistent read-back of it missed. Refusing
// here would fail a contributor whose account was in fact recorded, and dereferencing the
// record it did not return would panic.
func TestBindSigningIdentity_MissedReadBackAfterASuccessfulWriteStillSucceeds(t *testing.T) {
	own := &v1Models.User{UserID: "user-own", LfUsername: callerLFUsername}
	repo := &fakeRepo{byLFUsername: map[string][]*v1Models.User{callerLFUsername: {own}}}
	writer := newFakeUsersWriter(own)
	writer.updateReadBackMisses = true
	svc := newBindingService(repo, writer)

	result, err := bind(svc, callerContext())

	require.NoError(t, err)
	assert.Equal(t, "user-own", *result.UserID)
	assert.Equal(t, chosenGithubID, *result.GithubID)
}

// Resolution must key on the account number before the LF identity. The authentication
// middleware creates an LF-only record whenever its lookups miss, on this very request,
// so an LF-first order would bind the account to that fresh empty record while a
// record keyed on the account already existed.
func TestBindSigningIdentity_ResolutionIsGithubFirst(t *testing.T) {
	keyedOnAccount := &v1Models.User{
		UserID:     "user-github-keyed",
		LfUsername: callerLFUsername,
		GithubID:   formatGithubID(chosenGithubID),
	}
	freshEmptyRecord := &v1Models.User{UserID: "user-lf-only", LfUsername: callerLFUsername}

	repo := &fakeRepo{
		byGithubID:   map[int64][]*v1Models.User{chosenGithubID: {keyedOnAccount}},
		byLFUsername: map[string][]*v1Models.User{callerLFUsername: {freshEmptyRecord}},
	}
	writer := newFakeUsersWriter(keyedOnAccount, freshEmptyRecord)
	svc := newBindingService(repo, writer)

	result, err := bind(svc, callerContext())

	require.NoError(t, err)
	assert.Equal(t, "user-github-keyed", *result.UserID)
	assert.NotContains(t, writer.updates, "user-lf-only", "the empty LF-only record must not be bound")
}

func TestBindSigningIdentity_SeveralRecordsSharingTheLFIdentityRefuse(t *testing.T) {
	repo := &fakeRepo{byLFUsername: map[string][]*v1Models.User{
		callerLFUsername: {
			{UserID: "user-1", LfUsername: callerLFUsername},
			{UserID: "user-2", LfUsername: callerLFUsername},
		},
	}}
	writer := newFakeUsersWriter()
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	assertRefused(t, err, ReasonRecordConflict)
	assert.Empty(t, writer.updates)
	assert.Empty(t, writer.created)
}

func TestBindSigningIdentity_RenamedHandleIsRefreshedOnTheAccountNumber(t *testing.T) {
	record := &v1Models.User{
		UserID:         "user-1",
		LfUsername:     callerLFUsername,
		GithubID:       formatGithubID(chosenGithubID),
		GithubUsername: "old-handle",
	}
	repo := &fakeRepo{byGithubID: map[int64][]*v1Models.User{chosenGithubID: {record}}}
	writer := newFakeUsersWriter(record)
	svc := newBindingService(repo, writer)

	result, err := bind(svc, callerContext())

	require.NoError(t, err)
	// Matching is on the number, so a rename does not break resolution; the stored handle
	// is display data and is brought up to date without re-associating anything.
	assert.Equal(t, "octocat-work", result.GithubUsername)
	assert.NotContains(t, writer.updates["user-1"], "user_github_id")
}

// --- Stale index images ------------------------------------------------------------
//
// Both indexes this service resolves through are global secondary indexes, so both are
// allowed to serve an image the base table has already moved past. The two tests below
// hold the index and the base table deliberately out of step, which is the state neither
// a single-record fixture nor a happy-path test can reach.

// The account-number index still lists the record under the submitted account; the base
// table shows the record has since been rebound to another one. Refreshing the handle on
// that evidence would leave the record naming one account and numbered as another - and
// the handle is what CCLA approval lists are matched against, so the wrong one is not a
// display defect.
func TestBindSigningIdentity_StaleGithubIndexDoesNotOverwriteTheHandle(t *testing.T) {
	stale := &v1Models.User{
		UserID:         "user-1",
		LfUsername:     callerLFUsername,
		GithubID:       formatGithubID(chosenGithubID),
		GithubUsername: "old-handle",
	}
	current := &v1Models.User{
		UserID:         "user-1",
		LfUsername:     callerLFUsername,
		GithubID:       formatGithubID(otherGithubID),
		GithubUsername: "current-handle",
	}
	repo := &fakeRepo{
		byGithubID:     map[int64][]*v1Models.User{chosenGithubID: {stale}},
		consistentRead: func(string) (*v1Models.User, error) { return current, nil },
	}
	writer := newFakeUsersWriter(current)
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	assertRefused(t, err, ReasonRecordedMismatch)
	assert.Empty(t, writer.updates, "a record the base table says holds another account must not be written to")
	assert.Equal(t, "current-handle", current.GithubUsername)
}

// The LF-identity index shows the caller's record holding no account, so the adopt branch
// looks safe; the base table shows an account recorded on it already. Writing on the index
// image would overwrite that binding, and the post-write confirmation would then find the
// account it submitted and call it a success - so lf_record_already_bound would never fire
// for the one case it exists to catch.
func TestBindSigningIdentity_StaleLFIndexDoesNotOverwriteANewerBinding(t *testing.T) {
	stale := &v1Models.User{UserID: "user-own", LfUsername: callerLFUsername}
	current := &v1Models.User{
		UserID:     "user-own",
		LfUsername: callerLFUsername,
		GithubID:   formatGithubID(otherGithubID),
	}
	repo := &fakeRepo{
		byLFUsername:   map[string][]*v1Models.User{callerLFUsername: {stale}},
		consistentRead: func(string) (*v1Models.User, error) { return current, nil },
	}
	writer := newFakeUsersWriter(current)
	svc := newBindingService(repo, writer)

	_, err := bind(svc, callerContext())

	assertRefused(t, err, ReasonLFRecordAlreadyBound)
	assert.Empty(t, writer.updates, "the newer binding must survive a stale index read")
	assert.Equal(t, formatGithubID(otherGithubID), current.GithubID)
}

func TestBindSigningIdentity_ChoosingTheSecondAccountRecordsTheSecond(t *testing.T) {
	writer := newFakeUsersWriter()
	svc := newBindingService(&fakeRepo{}, writer)

	// Choosing the first would pass even if selection were ignored entirely, which is the
	// bug this feature exists to remove.
	result, err := bind(svc, callerContext())

	require.NoError(t, err)
	assert.Equal(t, chosenGithubID, *result.GithubID)
	assert.Equal(t, formatGithubID(chosenGithubID), writer.created[0].GithubID)
}
