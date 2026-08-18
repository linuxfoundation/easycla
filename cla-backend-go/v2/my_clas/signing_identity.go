// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package my_clas

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-openapi/strfmt"
	v1Models "github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/models"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
)

// Refusal reason codes. Each refusal carries its own code so that refusals can be counted
// by reason rather than in aggregate: a contested account and an unclaimed record both
// look like "binding failed", but they need different responses from different people, and
// a single counter cannot tell an operator which one is happening.
const (
	// ReasonIdentityUnavailable is returned when the request carries no LF identity to
	// associate the account with. Nothing is inferred from its absence.
	ReasonIdentityUnavailable = "identity_unavailable"
	// ReasonRecordConflict is returned when the account is held by a record belonging to
	// a different LF identity. The record is left untouched.
	ReasonRecordConflict = "record_conflict"
	// ReasonRecordUnclaimed is returned when the account is held by a record that names
	// nobody. Claiming it would need proof the caller owns the account, which this
	// service no longer has - see resolveExistingGithubRecord.
	ReasonRecordUnclaimed = "record_unclaimed"
	// ReasonDuplicateGithubID is returned when more than one record holds the account.
	// The index does not enforce uniqueness, so this is detected rather than assumed.
	ReasonDuplicateGithubID = "duplicate_github_id"
	// ReasonLFRecordAlreadyBound is returned when the caller's own record already holds a
	// different account. A record holds one account, so this is a refusal, not a swap.
	ReasonLFRecordAlreadyBound = "lf_record_already_bound"
	// ReasonRecordedMismatch is returned when what ended up on the record is not what was
	// submitted, which is the one check the caller cannot make for itself.
	ReasonRecordedMismatch = "recorded_mismatch"
	// ReasonIdentityMismatch is returned when the two identities on the request disagree -
	// the one the gateway authenticated and the one the token carries. It should never
	// fire; it is here because the write uses the token's identity, so a disagreement
	// would record the submitted GitHub account against a different person.
	ReasonIdentityMismatch = "identity_mismatch"
)

// Resolution outcomes, reported for observability. They are not behavioural for the caller.
const (
	outcomeMatched = "matched"
	outcomeCreated = "created"
	outcomeAdopted = "adopted"
)

// selectedAccount is the GitHub account the caller submitted. It is named for what it is -
// a selection made upstream - rather than for anything this service has established about
// it. Nothing here verifies that the caller owns it; that judgement is made by LFX Self
// Serve from the accounts Auth0 reports as linked, and this service records its word.
type selectedAccount struct {
	ID       int64
	Username string
}

// Refusal is a decision not to record an association, carrying the reason that decision
// was reached. Every refusal reachable before the write writes nothing: there is no
// partial-success path on those, because a refusal after a write would leave behind an
// association the evidence does not support.
//
// ReasonRecordedMismatch is the exception, and the only one. It is also raised after the
// write, when the record cannot be read back holding what was submitted, and it does not
// compensate that write - a second unconfirmed write on a record this service has just
// lost confidence about is not an improvement. Retrying is safe: a record already holding
// the submitted account resolves as outcomeMatched.
type Refusal struct {
	Reason  string
	Message string
}

func (r *Refusal) Error() string {
	return fmt.Sprintf("%s: %s", r.Reason, r.Message)
}

func refuse(reason, message string) *Refusal {
	return &Refusal{Reason: reason, Message: message}
}

// UsersWriter is the subset of the users service used to record a confirmed association.
// It is the users service rather than its repository so that the create and update both
// emit their existing user.created / user.updated events, which is what makes the
// association durably observable without a new mechanism.
type UsersWriter interface {
	CreateUser(user *v1Models.User, claUser *user.CLAUser) (*v1Models.User, error)
	UpdateUser(userID string, updates map[string]interface{}) (*v1Models.User, error)
	GetUser(userID string) (*v1Models.User, error)
}

// BindSigningIdentity records the GitHub account the caller submitted against the
// contributor's EasyCLA user record, and returns that record's identifier for the CLA
// hand-off to consume.
//
// The submitted account is taken on trust. Ownership is established upstream by LFX Self
// Serve, which offers only the accounts Auth0 reports the contributor has linked, and the
// caller is authenticated as that service. This endpoint therefore checks that recording
// the account is safe for records that already exist - it cannot check that the account is
// the caller's, and does not pretend to.
func (s *service) BindSigningIdentity(ctx context.Context, githubID int64, githubUsername string) (*models.SigningIdentity, error) {
	f := logrus.Fields{
		"functionName":   "v2.my_clas.signing_identity.BindSigningIdentity",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"githubID":       githubID,
		"githubUsername": githubUsername,
	}

	caller := user.VerifiedCallerFromContext(ctx)
	if caller == nil || caller.LFUsername == "" {
		// Without an LF identity there is nothing to associate the account with, and the
		// GitHub-first lookup below would fall through to a create that could not be
		// found again.
		return nil, s.refusal(f, ReasonIdentityUnavailable, "the request carries no LF identity to associate the GitHub account with")
	}
	f["lfUsername"] = caller.LFUsername

	selected := selectedAccount{ID: githubID, Username: githubUsername}

	record, outcome, err := s.resolveSigningRecord(ctx, f, caller, selected)
	if err != nil {
		return nil, err
	}
	if record == nil {
		// The write paths re-read the record they just wrote and report a miss as no
		// record and no error, so a resolution can succeed and still hand back nothing.
		// Dereferencing that below would panic on a request that had already written.
		return nil, s.refusal(f, ReasonRecordedMismatch, "the resolved user record could not be read back to confirm what was recorded")
	}

	// What was recorded must equal what was submitted. This is the one check the caller
	// cannot perform for itself, because only this side sees what the store accepted.
	//
	// Read consistently, on the base table. The lookup that resolved this record went
	// through a global secondary index, which is always eventually consistent, so the
	// record it returned may no longer hold the account it was indexed under - which is
	// exactly what this check is here to catch. An eventually consistent confirmation
	// could not tell that apart from its own stale view, and would refuse a contributor
	// whose write had in fact succeeded.
	confirmed, err := s.repo.GetUserByIDConsistent(ctx, record.UserID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to re-read the resolved user record to confirm what was recorded")
		return nil, err
	}
	if confirmed == nil || confirmed.GithubID != formatGithubID(selected.ID) {
		f["recordedGithubID"] = recordedGithubID(confirmed)
		return nil, s.refusal(f, ReasonRecordedMismatch, "the GitHub account recorded on the user record is not the account that was submitted")
	}

	// An empty identifier passes every check above and still breaks the hand-off: the
	// Console renders "invalid user ID in the URL", which reads as a broken product rather
	// than a failed resolution. Refuse here, where the reason is still known.
	if confirmed.UserID == "" {
		return nil, s.refusal(f, ReasonRecordedMismatch, "the resolved user record carries no identifier for the hand-off to use")
	}

	f["outcome"] = outcome
	f["userID"] = confirmed.UserID
	log.WithFields(f).Debug("recorded the signing GitHub identity")

	userID := confirmed.UserID
	accountID := selected.ID
	return &models.SigningIdentity{
		UserID:         &userID,
		GithubID:       &accountID,
		GithubUsername: confirmed.GithubUsername,
		Outcome:        outcome,
	}, nil
}

// resolveSigningRecord finds or creates the user record the submitted account belongs to.
//
// The lookup by account number comes before any lookup by LF identity, and the order is
// load-bearing rather than stylistic. The authentication middleware creates an LF-only
// record whenever its own lookups miss, unconditionally, and it runs before this handler
// on the same request - so an LF-first resolution would find that freshly created empty
// record and bind the account to it while a record keyed on that account already existed,
// producing exactly the two-records-one-account state this feature exists to prevent.
//
// Every outcome that is not clearly safe is a refusal. That is deliberate: a wrong guess
// here silently detaches a working association, and the contributor finds out through a
// pull-request check that stopped passing rather than through anything this service says.
// It matters more here than it would with provider-issued evidence, because the account
// number arriving on the request is the caller's word - so these checks, which reason only
// about what the store already holds, are the whole of what stands between a mistaken or
// forged selection and someone else's record.
func (s *service) resolveSigningRecord(ctx context.Context, f logrus.Fields, caller *user.CLAUser, selected selectedAccount) (*v1Models.User, string, error) {
	byGithub, err := s.repo.GetUsersByGithubID(ctx, selected.ID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to look up user records by GitHub account number")
		return nil, "", err
	}

	if len(byGithub) > 1 {
		// The index does not enforce uniqueness and the repository's own single-result
		// lookup only warns before returning the first match, so this has to be detected
		// deliberately. Resolving to whichever row happens to sort first would attach the
		// signature to an arbitrary one of two contested records.
		f["matchingRecords"] = len(byGithub)
		return nil, "", s.refusal(f, ReasonDuplicateGithubID, "more than one contributor record holds this GitHub account - the conflict needs resolving before it can be signed with")
	}

	if len(byGithub) == 1 {
		return s.resolveExistingGithubRecord(ctx, f, caller, selected, byGithub[0])
	}

	byLF, err := s.repo.GetUsersByLFUsername(ctx, caller.LFUsername)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to look up user records by LF identity")
		return nil, "", err
	}

	if len(byLF) == 0 {
		return s.createSigningRecord(f, caller, selected)
	}

	if len(byLF) > 1 {
		// Several records share this LF identity, so there is no single record to record
		// the account on and no basis for choosing between them. Picking one would bind
		// the account to a record the contributor may not be signing from.
		f["matchingRecords"] = len(byLF)
		return nil, "", s.refusal(f, ReasonRecordConflict, "more than one contributor record carries this LF identity - the conflict needs resolving before an account can be recorded")
	}

	return s.adoptSigningRecord(ctx, f, selected, byLF[0])
}

// resolveExistingGithubRecord handles the record already keyed on the submitted account.
func (s *service) resolveExistingGithubRecord(ctx context.Context, f logrus.Fields, caller *user.CLAUser, selected selectedAccount, record *v1Models.User) (*v1Models.User, string, error) {
	f["recordUserID"] = record.UserID

	if record.LfUsername == "" {
		// A record created by a GitHub-only flow: it holds the account but names nobody,
		// so the CLA never appears in anyone's list. Writing the caller's LF identity onto
		// it would reunite the contributor with their own signature history - and, if the
		// account was not theirs, would hand them somebody else's instead, because the
		// record carries signatures already.
		//
		// Doing that needs proof the caller owns the account. This service has none: the
		// account number is the caller's own submission. So the record is left untouched
		// and the reason is reported, rather than guessing on the odds that whoever asked
		// is the rightful owner.
		return nil, "", s.refusal(f, ReasonRecordUnclaimed, "this GitHub account is on a contributor record that names nobody, and claiming it needs proof of ownership this request cannot provide")
	}

	// Compared without regard to case, as the handler compares the two identities on the
	// request. A record written with different casing to the token's claim is the same
	// person, and refusing them here would lock them out of their own record.
	if !strings.EqualFold(record.LfUsername, caller.LFUsername) {
		// Reassigning an account two LF identities both claim would leave the losing
		// contributor's CLA no longer covering their commits, with nothing on screen to
		// point at. Reconciling a contested account is out of scope, and the
		// account-number index does not enforce uniqueness, so this is detected here
		// rather than prevented by the store.
		f["recordLfUsername"] = record.LfUsername
		return nil, "", s.refusal(f, ReasonRecordConflict, "this GitHub account is already associated with a different contributor record")
	}

	// The handle is display-only and drifts whenever the contributor renames themselves
	// on GitHub. Refreshing it is not a re-association - the account number, which is what
	// anything matches on, is unchanged and is not rewritten here.
	if selected.Username != "" && record.GithubUsername != selected.Username {
		// Everything decided above came from the account-number index, which is allowed
		// to serve an image the base table has already moved past. If this record has
		// since been rebound to another account, writing the submitted handle onto it
		// would leave the record naming one account and numbered as another - and the
		// handle is what approval lists match on, so the damage outlives the request.
		// Confirm against the base table before writing, not after.
		if err := s.confirmStillHolds(ctx, f, record.UserID, selected); err != nil {
			return nil, "", err
		}

		updated, err := s.usersWriter.UpdateUser(record.UserID, map[string]interface{}{
			"user_github_username": selected.Username,
		})
		if err != nil {
			log.WithFields(f).WithError(err).Warn("unable to refresh the stored GitHub handle")
			return nil, "", err
		}
		if updated == nil {
			// The update path re-reads the record it wrote without consistency, and
			// reports a miss as no record and no error. Only the identifier is needed
			// from it and that is unchanged, so carry the record already in hand and let
			// the consistent confirmation decide - refusing here would fail a
			// contributor whose write succeeded.
			return record, outcomeMatched, nil
		}
		return updated, outcomeMatched, nil
	}

	return record, outcomeMatched, nil
}

// createSigningRecord creates a contributor record carrying both the LF identity and the
// submitted account, for a contributor who has neither.
func (s *service) createSigningRecord(f logrus.Fields, caller *user.CLAUser, selected selectedAccount) (*v1Models.User, string, error) {
	// GithubID is a string on this model and the repository writes it as a DynamoDB
	// number, so a decimal string is the correct value at this seam. It is not a widening
	// of the account's type - see adoptSigningRecord for the seam where it matters.
	newUser := &v1Models.User{
		LfUsername:     caller.LFUsername,
		LfEmail:        strfmt.Email(caller.LFEmail),
		Username:       caller.Name,
		GithubID:       formatGithubID(selected.ID),
		GithubUsername: selected.Username,
	}

	created, err := s.usersWriter.CreateUser(newUser, caller)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to create the contributor record")
		return nil, "", err
	}

	return created, outcomeCreated, nil
}

// adoptSigningRecord records the submitted account on the caller's existing record, or
// refuses when that record already holds a different one.
func (s *service) adoptSigningRecord(ctx context.Context, f logrus.Fields, selected selectedAccount, record *v1Models.User) (*v1Models.User, string, error) {
	f["recordUserID"] = record.UserID

	// The record arrived from the LF-identity index, whose view of user_github_id can lag
	// the base table. Deciding "this record holds no account yet" from that view and then
	// writing unconditionally is how a binding made moments ago gets overwritten - and the
	// confirmation at the end of the request would see the account it submitted and call
	// it a success, so the refusal this branch exists for would never fire. Re-read the
	// record consistently and decide on that.
	if fresh, err := s.repo.GetUserByIDConsistent(ctx, record.UserID); err != nil {
		log.WithFields(f).WithError(err).Warn("unable to re-read the contributor record before recording the account")
		return nil, "", err
	} else if fresh != nil {
		record = fresh
	}

	if record.GithubID != "" && record.GithubID != formatGithubID(selected.ID) {
		// Overwriting here is the mistake this branch exists to prevent. A contributor
		// record holds one GitHub account, and replacing it detaches the previous one:
		// commits authored by it stop matching any record, and a CLA that worked
		// yesterday fails today with nothing on screen to point at.
		f["recordGithubID"] = record.GithubID
		return nil, "", s.refusal(f, ReasonLFRecordAlreadyBound, "this contributor record already has a different GitHub account recorded against it")
	}

	if record.GithubID == formatGithubID(selected.ID) {
		// The account-number index missed a record that in fact holds the account, which
		// a global secondary index is allowed to do briefly after a write. Nothing needs
		// recording.
		return record, outcomeMatched, nil
	}

	// The account number MUST be written as a Go integer. The update path marshals
	// whatever it is given, so a decimal string here would be stored as a DynamoDB string
	// attribute, which the numeric key condition on the account-number index would then
	// never match again - the association would look correct in the record and be
	// invisible to every lookup.
	updates := map[string]interface{}{"user_github_id": selected.ID}

	// A blank handle is absent, not a value to record. The handle keys its own index, so
	// an empty string is rejected outright as an index key - the whole write fails, and
	// the account never reaches the record. Were it accepted, it would replace whatever
	// handle the record already held with nothing.
	if selected.Username != "" {
		updates["user_github_username"] = selected.Username
	}

	updated, err := s.usersWriter.UpdateUser(record.UserID, updates)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to record the GitHub account on the contributor record")
		return nil, "", err
	}
	if updated == nil {
		// As in resolveExistingGithubRecord: a missed read-back after a write that
		// succeeded is not grounds to refuse. The identifier is unchanged, and the
		// consistent confirmation is what decides.
		return record, outcomeAdopted, nil
	}

	return updated, outcomeAdopted, nil
}

// confirmStillHolds re-reads a record on the base table and refuses unless it still holds
// the submitted account. It guards a write that an index-sourced decision is about to make
// on a record that index may be describing stale, so the read must be consistent to be
// worth performing at all.
func (s *service) confirmStillHolds(ctx context.Context, f logrus.Fields, userID string, selected selectedAccount) error {
	fresh, err := s.repo.GetUserByIDConsistent(ctx, userID)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to re-read the contributor record before writing to it")
		return err
	}
	if fresh == nil || fresh.GithubID != formatGithubID(selected.ID) {
		f["recordedGithubID"] = recordedGithubID(fresh)
		return s.refusal(f, ReasonRecordedMismatch, "the GitHub account recorded on the user record is not the account that was submitted")
	}
	return nil
}

// refusal logs a refusal under its own reason code and returns it. Logging here rather
// than at each call site is what keeps every refusal countable by reason.
func (s *service) refusal(f logrus.Fields, reason, message string) *Refusal {
	log.WithFields(f).WithField("refusalReason", reason).Warn(message)
	return refuse(reason, message)
}

// formatGithubID renders an account number the way the users repository and the v1 user
// model represent it - a decimal string that the repository converts to a DynamoDB number
// at the attribute boundary.
func formatGithubID(githubID int64) string {
	return strconv.FormatInt(githubID, 10)
}

func recordedGithubID(record *v1Models.User) string {
	if record == nil {
		return ""
	}
	return record.GithubID
}
