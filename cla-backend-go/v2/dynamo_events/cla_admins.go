// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package dynamo_events

import (
	"context"
	"errors"
	"fmt"
	"html"
	"sort"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/sirupsen/logrus"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/cla_admins"
	member_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/member-service"
)

// claAdminsPublisher publishes one organization's CLA-manager roster to member-service; signatureIDs name
// the rows this event changed, which are read from the base table even when the index lags.
type claAdminsPublisher interface {
	Publish(ctx context.Context, sfid string, signatureIDs ...string) (*cla_admins.Result, error)
}

// claAdminsAlerts reports a roster that could not be published (utils.EmailSender satisfies it).
type claAdminsAlerts interface {
	SendEmail(subject string, body string, recipients []string) error
}

// AlertRecipients splits a comma-separated address list such as PLATFORM_MAINTAINERS.
func AlertRecipients(raw string) []string {
	var out []string
	for _, v := range strings.Split(raw, ",") {
		if v = strings.TrimSpace(v); v != "" {
			out = append(out, v)
		}
	}
	return out
}

// Option configures optional stream handlers of NewService.
type Option func(s *service)

// WithCLAAdmins registers the CLA-manager publisher on the signatures stream (lfx-self-serve#4435); a
// publish that fails for good is reported to recipients through alerts when both are set.
func WithCLAAdmins(publisher claAdminsPublisher, alerts claAdminsAlerts, recipients []string) Option {
	return func(s *service) {
		s.claAdmins = publisher
		s.claAdminsAlerts = alerts
		s.claAdminsRecipients = recipients
		s.registerCallback(s.signaturesTable, Insert, s.CLAAdminsEvent)
		s.registerCallback(s.signaturesTable, Modify, s.CLAAdminsEvent)
		s.registerCallback(s.signaturesTable, Remove, s.CLAAdminsEvent)
	}
}

// claAdminsTarget is one organization (by company id) whose roster an event affects and the row that changed.
type claAdminsTarget struct {
	companyID   string
	signatureID string
}

// CLAAdminsEvent republishes the roster of every organization a corporate CLA row change affects: its
// company when the row appears, disappears, flips its signed flag or changes its signature_acl, and both
// the old and the new company when the row moves between them.
func (s *service) CLAAdminsEvent(event events.DynamoDBEventRecord) error {
	if s.claAdmins == nil {
		return nil
	}
	ctx := utils.NewContext()
	f := logrus.Fields{
		"functionName":   "v2.dynamo_events.CLAAdminsEvent",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"eventName":      event.EventName,
	}
	var oldSig, newSig Signature
	if len(event.Change.OldImage) > 0 {
		if err := unmarshalStreamImage(event.Change.OldImage, &oldSig); err != nil {
			log.WithFields(f).WithError(err).Warn("problem decoding pre-update signature")
			return err
		}
	}
	if len(event.Change.NewImage) > 0 {
		if err := unmarshalStreamImage(event.Change.NewImage, &newSig); err != nil {
			log.WithFields(f).WithError(err).Warn("problem decoding post-update signature")
			return err
		}
	}
	var firstErr error
	for _, target := range claAdminsTargets(oldSig, newSig) {
		if err := s.publishCLAAdmins(ctx, f, target); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *service) publishCLAAdmins(ctx context.Context, f logrus.Fields, target claAdminsTarget) error {
	tf := logrus.Fields{"signatureID": target.signatureID, "companyID": target.companyID}
	for k, v := range f {
		tf[k] = v
	}
	companyModel, err := s.companyRepo.GetCompany(ctx, target.companyID)
	if err != nil {
		log.WithFields(tf).WithError(err).Warn("unable to load the company of the corporate CLA - roster not published")
		s.alertCLAAdmins(tf, target, "", err)
		return err
	}
	if companyModel == nil || companyModel.CompanyExternalID == "" {
		log.WithFields(tf).Debug("company has no Salesforce account id - no roster to publish")
		return nil
	}
	tf["sfid"] = companyModel.CompanyExternalID
	res, err := s.claAdmins.Publish(ctx, companyModel.CompanyExternalID, target.signatureID)
	if errors.Is(err, member_service.ErrInvalidSFID) {
		log.WithFields(tf).Debug("company id is not a Salesforce account id - no roster to publish")
		return nil
	}
	if err != nil {
		log.WithFields(tf).WithError(err).Warn("publishing the CLA-manager roster failed")
		s.alertCLAAdmins(tf, target, companyModel.CompanyExternalID, err)
		return err
	}
	if res.Skipped {
		log.WithFields(tf).Info("organization is not a b2b_org yet - roster skipped, the backfill picks it up later")
		return nil
	}
	log.WithFields(tf).Infof("CLA-manager roster published: %d managers, retried=%t", len(res.Roster.Admins), res.Retried)
	return nil
}

// alertCLAAdmins emails the recipients about a roster that stays stale until the backfill runs: the
// stream does not retry a callback error and the worker raises no alarm on its own.
func (s *service) alertCLAAdmins(f logrus.Fields, target claAdminsTarget, sfid string, cause error) {
	if s.claAdminsAlerts == nil || len(s.claAdminsRecipients) == 0 {
		log.WithFields(f).Warn("no CLA-manager roster alert recipients configured - the failure is only logged")
		return
	}
	organization := sfid
	if organization == "" {
		organization = "of company " + target.companyID
	}
	subject := fmt.Sprintf("EasyCLA: CLA-manager roster not published for organization %s", organization)
	body := fmt.Sprintf("<p>EasyCLA could not publish the CLA-manager roster of organization %s to member-service.</p>"+
		"<p>Company ID: %s<br/>Signature ID: %s<br/>Error: %s</p>"+
		"<p>The roster stays stale until <code>org_import cla-admins --apply</code> runs for it.</p>",
		html.EscapeString(organization), html.EscapeString(target.companyID), html.EscapeString(target.signatureID), html.EscapeString(cause.Error()))
	if err := s.claAdminsAlerts.SendEmail(subject, body, s.claAdminsRecipients); err != nil {
		log.WithFields(f).WithError(err).Warn("unable to send the CLA-manager roster alert")
		return
	}
	log.WithFields(f).Infof("CLA-manager roster alert sent to %d recipient(s)", len(s.claAdminsRecipients))
}

// claAdminsTargets lists the organizations whose roster the row change can alter; metadata-only changes
// yield none. The row's current state is read at publish time, the images only decide whom to refresh.
func claAdminsTargets(oldSig, newSig Signature) []claAdminsTarget {
	oldOK := eligibleCCLA(oldSig)
	newOK := eligibleCCLA(newSig)
	if oldOK && newOK && oldSig.SignatureReferenceID == newSig.SignatureReferenceID {
		if sameMembers(oldSig.SignatureACL, newSig.SignatureACL) {
			return nil
		}
		return []claAdminsTarget{{companyID: newSig.SignatureReferenceID, signatureID: newSig.SignatureID}}
	}
	var targets []claAdminsTarget
	if oldOK {
		targets = append(targets, claAdminsTarget{companyID: oldSig.SignatureReferenceID, signatureID: oldSig.SignatureID})
	}
	if newOK {
		targets = append(targets, claAdminsTarget{companyID: newSig.SignatureReferenceID, signatureID: newSig.SignatureID})
	}
	return targets
}

func eligibleCCLA(sig Signature) bool {
	return sig.SignatureType == CCLASignatureType && sig.SignatureReferenceType == utils.SignatureReferenceTypeCompany && sig.SignatureSigned
}

func sameMembers(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}
