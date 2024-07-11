// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package signatures

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/communitybridge/easycla/cla-backend-go/project/service"

	"github.com/LF-Engineering/lfx-kit/auth"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/aws/aws-sdk-go/aws"

	"github.com/communitybridge/easycla/cla-backend-go/events"
	"github.com/communitybridge/easycla/cla-backend-go/projects_cla_groups"

	"github.com/jinzhu/copier"

	"github.com/communitybridge/easycla/cla-backend-go/company"
	v1Models "github.com/communitybridge/easycla/cla-backend-go/gen/v1/models"
	"github.com/communitybridge/easycla/cla-backend-go/gen/v2/models"
	v2Sigs "github.com/communitybridge/easycla/cla-backend-go/gen/v2/restapi/operations/signatures"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
	"github.com/communitybridge/easycla/cla-backend-go/signatures"
	"github.com/communitybridge/easycla/cla-backend-go/users"
	"github.com/communitybridge/easycla/cla-backend-go/utils"
	"github.com/communitybridge/easycla/cla-backend-go/v2/approvals"
	"github.com/sirupsen/logrus"
)

// constants
const (
	// HugePageSize constant for querying signatures
	HugePageSize = int64(10000)
)

var (
	// ErrZipNotPresent error
	ErrZipNotPresent = errors.New("zip file not present")
)

// ServiceInterface contains method of v2 signature service
type ServiceInterface interface {
	GetProjectCompanySignatures(ctx context.Context, companyID, companySFID, projectSFID string) (*models.CorporateSignatures, error)
	GetProjectIclaSignaturesCsv(ctx context.Context, claGroupID string) ([]byte, error)
	GetProjectCclaSignaturesCsv(ctx context.Context, claGroupID string) ([]byte, error)
	GetProjectIclaSignatures(ctx context.Context, claGroupID string, searchTerm *string, approved, signed *bool, pageSize int64, nextKey string, withExtraDetails bool) (*models.IclaSignatures, error)
	GetClaGroupCorporateContributorsCsv(ctx context.Context, claGroupID string, companyID string) ([]byte, error)
	GetClaGroupCorporateContributors(ctx context.Context, params v2Sigs.ListClaGroupCorporateContributorsParams) (*models.CorporateContributorList, error)
	GetSignedDocument(ctx context.Context, signatureID string) (*models.SignedDocument, error)
	GetSignedIclaZipPdf(claGroupID string) (*models.URLObject, error)
	GetSignedCclaZipPdf(claGroupID string) (*models.URLObject, error)
	InvalidateICLA(ctx context.Context, claGroupID string, userID string, authUser *auth.User, eventsService events.Service, eventArgs *events.LogEventArgs) error
	EclaAutoCreate(ctx context.Context, signatureID string, autoCreateECLA bool) error
	IsUserAuthorized(ctx context.Context, lfid, claGroupId string) (*models.LfidAuthorizedResponse, error)
}

// Service structure/model
type Service struct {
	v1ProjectService      service.Service
	v1CompanyService      company.IService
	v1SignatureService    signatures.SignatureService
	v1SignatureRepo       signatures.SignatureRepository
	usersService          users.Service
	projectsClaGroupsRepo projects_cla_groups.Repository
	s3                    *s3.S3
	signaturesBucket      string
	approvalsRepos        approvals.IRepository
}

// NewService creates instance of v2 signature service
func NewService(awsSession *session.Session, signaturesBucketName string, v1ProjectService service.Service,
	v1CompanyService company.IService,
	v1SignatureService signatures.SignatureService,
	pcgRepo projects_cla_groups.Repository, v1SignatureRepo signatures.SignatureRepository, usersService users.Service, approvalsRepo approvals.IRepository) *Service {
	return &Service{
		v1ProjectService:      v1ProjectService,
		v1CompanyService:      v1CompanyService,
		v1SignatureService:    v1SignatureService,
		v1SignatureRepo:       v1SignatureRepo,
		usersService:          usersService,
		projectsClaGroupsRepo: pcgRepo,
		s3:                    s3.New(awsSession),
		signaturesBucket:      signaturesBucketName,
		approvalsRepos:        approvalsRepo,
	}
}

// GetProjectCompanySignatures return the signatures for the specified project and company information
func (s *Service) GetProjectCompanySignatures(ctx context.Context, companyID, companySFID, projectSFID string) (*models.CorporateSignatures, error) {
	pm, err := s.projectsClaGroupsRepo.GetClaGroupIDForProject(ctx, projectSFID)
	if err != nil {
		return nil, err
	}
	signed := true
	approved := true
	sig, err := s.v1SignatureService.GetProjectCompanySignature(ctx, companyID, pm.ClaGroupID, &signed, &approved, nil, aws.Int64(HugePageSize))
	if err != nil {
		return nil, err
	}
	resp := &v1Models.Signatures{
		Signatures: make([]*v1Models.Signature, 0),
	}
	if sig != nil {
		resp.ResultCount = 1
		resp.TotalCount = 1
		resp.ProjectID = sig.ProjectID
		resp.Signatures = append(resp.Signatures, sig)
	}
	oldformatSignatures, err := v2SignaturesReplaceCompanyID(resp, companyID, companySFID)
	if err != nil {
		return nil, err
	}

	return s.v2SignaturesToCorporateSignatures(*oldformatSignatures, projectSFID)
}

// eclaSigCsvLine returns a single ECLA signature CSV line
func eclaSigCsvLine(sig *v1Models.CorporateContributor) string {
	var dateTime string
	t, err := utils.ParseDateTime(sig.Timestamp)
	if err != nil {
		log.WithFields(logrus.Fields{"signature_id": sig.LinuxFoundationID, "signature_created": sig.Timestamp}).
			Error("invalid time format present for signatures")
	} else {
		dateTime = t.Format("Jan 2,2006")
	}
	return fmt.Sprintf("\n%s,%s,%s,%s,\"%s\"", sig.GithubID, sig.LinuxFoundationID, sig.Name, sig.Email, dateTime)
}

// GetClaGroupCorporateContributorsCsv returns the CLA Group corporate contributors as a CSV
func (s *Service) GetClaGroupCorporateContributorsCsv(ctx context.Context, claGroupID string, companyID string) ([]byte, error) {
	var b bytes.Buffer
	result, err := s.v1SignatureService.GetClaGroupCorporateContributors(ctx, claGroupID, &companyID, nil, nil, nil)
	if err != nil {
		return nil, err
	}

	if len(result.List) == 0 {
		return nil, errors.New("not Found")
	}

	b.WriteString(`GitHub ID,LF_ID,Name,Email,Date Signed`)
	for _, sig := range result.List {
		b.WriteString(eclaSigCsvLine(sig))
	}
	return b.Bytes(), nil
}

// GetProjectIclaSignaturesCsv returns the ICLA signatures as a CSV file for the specified CLA Group
func (s *Service) GetProjectIclaSignaturesCsv(ctx context.Context, claGroupID string) ([]byte, error) {
	f := logrus.Fields{
		"functionName":   "v2.signature_service.GetProjectIclaSignaturesCsv",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
	}

	var totalResults []*v1Models.IclaSignature
	lastKeyScanned := ""
	batchSize := int64(500)
	loadUserDetails := true
	// Loop until we have all the results - 100 per page
	for {
		log.WithFields(f).Debugf("loading ICLAs - %d of %d so far - requesting page with lastKeyScanned: %s", batchSize, len(totalResults), lastKeyScanned)
		result, err := s.v1SignatureService.GetClaGroupICLASignatures(ctx, claGroupID, nil, nil, nil, batchSize, lastKeyScanned, loadUserDetails)
		if err != nil {
			return nil, err
		}
		totalResults = append(totalResults, result.List...)

		if result.LastKeyScanned == "" {
			break
		}

		lastKeyScanned = result.LastKeyScanned
	}

	var b bytes.Buffer
	b.WriteString(iclaSigCsvHeader())
	for _, sig := range totalResults {
		b.WriteString(iclaSigCsvLine(sig))
	}
	return b.Bytes(), nil
}

// GetProjectCclaSignaturesCsv returns the ICLA signatures as a CSV file for the specified CLA Group and search term filters
func (s *Service) GetProjectCclaSignaturesCsv(ctx context.Context, claGroupID string) ([]byte, error) {
	f := logrus.Fields{
		"functionName":   "v2.signatures.service.GetProjectCclaSignaturesCsv",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
	}
	log.WithFields(f).Debug("querying for CCLA signatures...")
	result, err := s.v1SignatureService.GetClaGroupCCLASignatures(ctx, claGroupID, nil, nil)
	if err != nil {
		log.WithFields(f).Warnf("error loading CCLA signatures for CLA group, error: %+v", err)
		return nil, err
	}
	log.WithFields(f).Debugf("loaded %d CCLA signatures", len(result.Signatures))

	var b bytes.Buffer
	log.WithFields(f).Debug("writing CCLA signatures header CSV...")
	b.WriteString(cclaSigCsvHeader())
	log.WithFields(f).Debugf("writing CCLA %d signatures records as CSV...", len(result.Signatures))
	for _, sig := range result.Signatures {
		b.WriteString(cclaSigCsvLine(sig))
	}
	return b.Bytes(), nil
}

// GetProjectIclaSignatures returns the ICLA signatures for the specified CLA Group and search term filters
func (s *Service) GetProjectIclaSignatures(ctx context.Context, claGroupID string, searchTerm *string, approved, signed *bool, pageSize int64, nextKey string, withExtraDetails bool) (*models.IclaSignatures, error) {
	f := logrus.Fields{
		"functionName":     "v2.signatures.service.GetProjectIclaSignatures",
		utils.XREQUESTID:   ctx.Value(utils.XREQUESTID),
		"claGroupID":       claGroupID,
		"searchTerm":       utils.StringValue(searchTerm),
		"approved":         utils.BoolValue(approved),
		"signed":           utils.BoolValue(signed),
		"withExtraDetails": withExtraDetails,
	}

	var out models.IclaSignatures
	result, err := s.v1SignatureService.GetClaGroupICLASignatures(ctx, claGroupID, searchTerm, approved, signed, pageSize, nextKey, withExtraDetails)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to load ICLA signatures using the specified search parameters")
		return nil, err
	}

	err = copier.Copy(&out, result)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("unable to convert signature results from v1 to v2")
		return nil, err
	}

	return &out, nil
}

// GetSignedDocument returns the signed document for the specified signature ID
func (s *Service) GetSignedDocument(ctx context.Context, signatureID string) (*models.SignedDocument, error) {
	sig, err := s.v1SignatureService.GetSignature(ctx, signatureID)
	if err != nil {
		return nil, err
	}
	if sig.SignatureType == utils.SignatureTypeCLA && sig.CompanyName != "" {
		return nil, errors.New("bad request. employee signature does not have signed document")
	}
	var url string
	switch sig.SignatureType {
	case utils.SignatureTypeCLA:
		url = utils.SignedCLAFilename(sig.ProjectID, "icla", sig.SignatureReferenceID, sig.SignatureID)
	case utils.SignatureTypeCCLA:
		url = utils.SignedCLAFilename(sig.ProjectID, "ccla", sig.SignatureReferenceID, sig.SignatureID)
	}
	signedURL, err := utils.GetDownloadLink(url)
	if err != nil {
		return nil, err
	}
	return &models.SignedDocument{
		SignatureID:  signatureID,
		SignedClaURL: signedURL,
	}, nil
}

// GetSignedCclaZipPdf returns the signed CCLA Zip PDF reference
func (s *Service) GetSignedCclaZipPdf(claGroupID string) (*models.URLObject, error) {
	url := utils.SignedClaGroupZipFilename(claGroupID, utils.ClaTypeCCLA)
	ok, err := s.IsZipPresentOnS3(url)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrZipNotPresent
	}
	signedURL, err := utils.GetDownloadLink(url)
	if err != nil {
		return nil, err
	}
	return &models.URLObject{
		URL: signedURL,
	}, nil
}

// GetSignedIclaZipPdf returns the signed ICLA Zip PDF reference
func (s *Service) GetSignedIclaZipPdf(claGroupID string) (*models.URLObject, error) {
	url := utils.SignedClaGroupZipFilename(claGroupID, utils.ClaTypeICLA)
	ok, err := s.IsZipPresentOnS3(url)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrZipNotPresent
	}
	signedURL, err := utils.GetDownloadLink(url)
	if err != nil {
		return nil, err
	}
	return &models.URLObject{
		URL: signedURL,
	}, nil
}

// IsZipPresentOnS3 returns true if the specified file is present in S3
func (s *Service) IsZipPresentOnS3(zipFilePath string) (bool, error) {
	_, err := s.s3.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.signaturesBucket),
		Key:    aws.String(zipFilePath),
	})
	if err != nil {
		aerr, ok := err.(awserr.Error)
		if ok && aerr.Code() == s3.ErrCodeNoSuchKey {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetClaGroupCorporateContributors returns the list of corporate contributors for the specified CLA Group and company
func (s *Service) GetClaGroupCorporateContributors(ctx context.Context, params v2Sigs.ListClaGroupCorporateContributorsParams) (*models.CorporateContributorList, error) {
	f := logrus.Fields{
		"functionName": "v2.signatures.service.GetClaGroupCorporateContributors",
		"claGroupID":   params.ClaGroupID,
		"companyID":    params.CompanyID,
	}
	if params.SearchTerm != nil {
		f["searchTerm"] = *params.SearchTerm
	}

	log.WithFields(f).Debug("querying CLA corporate contributors...")
	result, err := s.v1SignatureService.GetClaGroupCorporateContributors(ctx, params.ClaGroupID, params.CompanyID, params.PageSize, params.NextKey, params.SearchTerm)
	if err != nil {
		return nil, err
	}
	log.WithFields(f).Debugf("discovered %d CLA corporate contributors...", len(result.List))

	log.WithFields(f).Debug("converting to v2 response model...")
	var resp models.CorporateContributorList
	err = copier.Copy(&resp, result)
	if err != nil {
		return nil, err
	}

	return &resp, nil
}

// InvalidateICLA invalidates the specified signature record using the supplied parameters
func (s *Service) InvalidateICLA(ctx context.Context, claGroupID string, userID string, authUser *auth.User, eventsService events.Service, eventArgs *events.LogEventArgs) error {
	f := logrus.Fields{
		"functionName": "v2.signatures.service.InvalidateICLA",
		"claGroupID":   claGroupID,
		"userID":       userID,
	}
	// Get signature record
	log.WithFields(f).Debug("getting signature record ...")
	approved, signed := true, true
	icla, iclaErr := s.v1SignatureService.GetIndividualSignature(ctx, claGroupID, userID, &approved, &signed)
	if iclaErr != nil {
		log.WithFields(f).Debug("unable to get individual signature")
		return iclaErr
	}

	// Get cla Group
	log.WithFields(f).Debug("getting clGroup...")
	claGroup, claGrpErr := s.v1ProjectService.GetCLAGroupByID(ctx, claGroupID)
	if claGrpErr != nil {
		log.WithFields(f).Debug("unable to fetch cla Group record")
		return claGrpErr
	}

	//Get user record
	user, userErr := s.usersService.GetUser(userID)
	if userErr != nil {
		log.WithFields(f).Debug("unable to get user record")
		return userErr
	}

	log.WithFields(f).Debug("invalidating signature record ...")
	note := fmt.Sprintf("Signature invalidated (approved set to false) by %s for %s ", authUser.UserName, utils.GetBestUsername(user))
	err := s.v1SignatureRepo.InvalidateProjectRecord(ctx, icla.SignatureID, note)
	if err != nil {
		log.WithFields(f).Debug("unable to invalidate icla record")
		return err
	}
	// send email
	email := utils.GetBestEmail(user)
	log.WithFields(f).Debugf("sending invalidation email to : %s ", email)
	subject := fmt.Sprintf("EasyCLA: ICLA invalidated for %s ", claGroup.ProjectName)
	params := signatures.InvalidateSignatureTemplateParams{
		RecipientName:  utils.GetBestUsername(user),
		ProjectManager: authUser.UserName,
		CLAGroupName:   claGroup.ProjectName,
	}
	body, renderErr := utils.RenderTemplate(claGroup.Version, signatures.InvalidateICLASignatureTemplateName, signatures.InvalidateICLASignatureTemplate, params)
	if renderErr != nil {
		log.WithFields(f).Debugf("unable to render email approval template for user: %s ", email)
	} else {
		err := utils.SendEmail(subject, body, []string{email})
		if err != nil {
			log.WithFields(f).Debugf("unable to send approval list update email to : %s ", email)
		}
	}

	eventArgs.UserName = utils.GetBestUsername(user)
	eventArgs.UserModel = user
	eventArgs.ProjectName = claGroup.ProjectName

	// Log event
	eventsService.LogEventWithContext(ctx, eventArgs)

	return nil
}

// EclaAutoCreate this routine updates the CCLA signature record by adjusting the auto_create_ecla column to the specified value
func (s *Service) EclaAutoCreate(ctx context.Context, signatureID string, autoCreateECLA bool) error {
	f := logrus.Fields{
		"functionName":   "v2.signatures.service.EclaAutoCreate",
		"signatureID":    signatureID,
		"autoCreateECLA": autoCreateECLA,
	}

	log.WithFields(f).Debug("updating CCLA signature record for auto_create_ecla...")
	err := s.v1SignatureRepo.EclaAutoCreate(ctx, signatureID, autoCreateECLA)
	if err != nil {
		log.WithFields(f).Debug("unable to update CCLA signature record for auto_create_ecla")
		return err
	}

	return nil
}

func (s *Service) IsUserAuthorized(ctx context.Context, lfid, claGroupId string) (*models.LfidAuthorizedResponse, error) {
	f := logrus.Fields{
		"functionName":   "v2.signatures.service.IsUserAuthorized",
		"lfid":           lfid,
		"claGroupId":     claGroupId,
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}

	hasSigned := false

	response := models.LfidAuthorizedResponse{
		ClaGroupID:         claGroupId,
		Lfid:               lfid,
		Authorized:         false,
		ICLA:               false,
		CCLA:               false,
		CCLARequiresICLA:   false,
		CompanyAffiliation: false,
	}

	// fetch cla group
	log.WithFields(f).Debug("fetching cla group")
	claGroup, err := s.v1ProjectService.GetCLAGroupByID(ctx, claGroupId)
	if err != nil {
		log.WithFields(f).WithError(err).Debug("unable to fetch cla group")
		return nil, err
	}

	if claGroup == nil {
		log.WithFields(f).Debug("cla group not found")
		return &response, nil
	}
	response.CCLARequiresICLA = claGroup.ProjectCCLARequiresICLA

	// fetch cla user
	log.WithFields(f).Debug("fetching user by lfid")
	user, err := s.usersService.GetUserByLFUserName(lfid)

	if err != nil {
		log.WithFields(f).WithError(err).Debug("unable to fetch lfusername")
		return nil, err
	}

	if user == nil {
		log.WithFields(f).Debug("user not found")
		return &response, nil
	}

	// check if user has signed ICLA
	log.WithFields(f).Debug("checking if user has signed ICLA")
	approved, signed := true, true
	icla, iclaErr := s.v1SignatureService.GetIndividualSignature(ctx, claGroupId, user.UserID, &approved, &signed)
	if iclaErr != nil {
		log.WithFields(f).WithError(iclaErr).Debug("unable to get individual signature")
	}

	if icla != nil {
		log.WithFields(f).Debug("user has signed ICLA")
		response.ICLA = true
		hasSigned = true
	} else {
		log.WithFields(f).Debug("user has not signed ICLA")
	}

	// fetch company
	if user.CompanyID == "" {
		log.WithFields(f).Debug("user company id not found")
		response.CompanyAffiliation = false
	} else {
		log.WithFields(f).Debug("fetching company")
		companyModel, err := s.v1CompanyService.GetCompany(ctx, user.CompanyID)
		if companyErr, ok := err.(*utils.CompanyNotFound); ok {
			log.WithFields(f).WithError(companyErr).Debug("company not found")
			response.CompanyAffiliation = false
		} else if err != nil {
			log.WithFields(f).WithError(err).Debug("unable to fetch company")
			return nil, err
		} else {
			log.WithFields(f).Debug("company found")
			response.CompanyAffiliation = true
			// process ecla
			ecla, err := s.v1SignatureService.ProcessEmployeeSignature(ctx, companyModel, claGroup, user)
			if err != nil {
				log.WithFields(f).WithError(err).Debug("unable to process ecla")
				return nil, err
			}
			if ecla != nil && *ecla {
				log.WithFields(f).Debug("user has signed ECLA")
				hasSigned = true
				response.CCLA = true
			} else {
				log.WithFields(f).Debug("user has not acknowledged with the company ")
			}
		}
	}

	response.Authorized = hasSigned
	return &response, nil
}
