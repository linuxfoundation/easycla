// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package template

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	"github.com/sirupsen/logrus"
	"golang.org/x/sync/errgroup"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/linuxfoundation/easycla/cla-backend-go/docraptor"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/aymerick/raymond"
)

const (
	claTypeICLA = "icla"
	claTypeCCLA = "ccla"
)

// ServiceInterface interface
type ServiceInterface interface {
	GetTemplates(ctx context.Context) ([]models.Template, error)
	GetTemplateName(ctx context.Context, templateID string) (string, error)
	CreateCLAGroupTemplate(ctx context.Context, claGroupID string, claGroupFields *models.CreateClaGroupTemplate) (models.TemplatePdfs, error)
	CreateTemplatePreview(ctx context.Context, claGroupFields *models.CreateClaGroupTemplate, templateFor string) ([]byte, error)
	GetCLATemplatePreview(ctx context.Context, claGroupID, claType string, watermark bool) ([]byte, error)
	CLAGroupTemplateExists(ctx context.Context, templateID string) bool
}

// Service object/struct
type Service struct {
	stage           string // The AWS stage (dev, staging, prod)
	templateRepo    RepositoryInterface
	docRaptorClient docraptor.Client
	s3Client        *s3manager.Uploader
}

// NewService API call
func NewService(stage string, templateRepo RepositoryInterface, docRaptorClient docraptor.Client, awsSession *session.Session) Service {
	return Service{
		stage:           stage,
		templateRepo:    templateRepo,
		docRaptorClient: docRaptorClient,
		s3Client:        s3manager.NewUploader(awsSession),
	}
}

// GetTemplates API call
func (s Service) GetTemplates(ctx context.Context) ([]models.Template, error) {
	f := logrus.Fields{
		"functionName":   "v1.template.service.GetTemplates",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	log.WithFields(f).Debug("Loading templates...")
	templates, err := s.templateRepo.GetTemplates(ctx)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem loading templates...")
		return nil, err
	}

	// Remove HTML from template
	for i, template := range templates {
		template.IclaHTMLBody = ""
		template.CclaHTMLBody = ""
		templates[i] = template
	}

	return templates, nil
}

// GetTemplateName returns the template name when provided the template ID
func (s Service) GetTemplateName(ctx context.Context, templateID string) (string, error) {
	f := logrus.Fields{
		"functionName":   "v1.template.service.GetTemplateName",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"templateID":     templateID,
	}
	templateName, err := s.templateRepo.GetTemplateName(ctx, templateID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem loading template by ID: %s", templateID)
		return "", err
	}

	return templateName, nil
}

// CreateTemplatePreview returns a PDF using the specified CLA Group field values and template identifier
func (s Service) CreateTemplatePreview(ctx context.Context, claGroupFields *models.CreateClaGroupTemplate, templateFor string) ([]byte, error) {
	f := logrus.Fields{
		"functionName":   "v1.template.service.CreateTemplatePreview",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"templateID":     claGroupFields.TemplateID,
		"templateFor":    templateFor,
	}
	var template models.Template
	var err error

	templateID := ApacheStyleTemplateID
	if claGroupFields.TemplateID != "" {
		templateID = claGroupFields.TemplateID
	}
	log.WithFields(f).Debugf("using template ID: %s", templateID)

	// Get Template
	template, err = s.templateRepo.GetTemplate(templateID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to fetch template fields : %s",
			claGroupFields.TemplateID)
		return nil, err
	}
	log.WithFields(f).Debugf("loaded template ID: %s with ID: %s", template.Name, template.ID)

	// Apply template fields
	iclaTemplateHTML, cclaTemplateHTML, err := s.InjectProjectInformationIntoTemplate(template, claGroupFields.MetaFields)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to inject metadata details into template")
		return nil, err
	}
	var templateHTML string
	switch templateFor {
	case utils.ClaTypeICLA:
		templateHTML = iclaTemplateHTML
	case utils.ClaTypeCCLA:
		templateHTML = cclaTemplateHTML
	default:
		return nil, errors.New("invalid value of template_for")
	}

	ioReader, err := s.docRaptorClient.CreatePDF(templateHTML, templateFor)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem with API call to docraptor service")
		return nil, err
	}
	defer func() {
		closeErr := ioReader.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing PDF")
		}
	}()

	bytes, err := io.ReadAll(ioReader)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("error reading PDF bytes from the generated template")
		return nil, err
	}

	return bytes, err
}

// CreateCLAGroupTemplate service method
func (s Service) CreateCLAGroupTemplate(ctx context.Context, claGroupID string, claGroupFields *models.CreateClaGroupTemplate) (models.TemplatePdfs, error) {
	f := logrus.Fields{
		"functionName":   "v1.template.service.CreateCLAGroupTemplate",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"claGroupFields": claGroupFields,
	}

	// Verify claGroupID matches an existing CLA Group
	claGroup, err := s.templateRepo.GetCLAGroup(claGroupID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to fetch CLA group by id: %s - returning empty template PDFs", claGroupID)
		return models.TemplatePdfs{}, err
	}

	// Get Template
	template, err := s.templateRepo.GetTemplate(claGroupFields.TemplateID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Unable to fetch template id: %s - returning empty template PDFs",
			claGroupFields.TemplateID)
		return models.TemplatePdfs{}, err
	}

	// Apply template fields
	iclaTemplateHTML, cclaTemplateHTML, err := s.InjectProjectInformationIntoTemplate(template, claGroupFields.MetaFields)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("Unable to inject metadata details into template - returning empty template PDFs")
		return models.TemplatePdfs{}, err
	}

	bucket := fmt.Sprintf("cla-signature-files-%s", s.stage)

	// Create PDF
	var pdfUrls models.TemplatePdfs
	var iclaFileURL string
	var cclaFileURL string

	// Use an error group to keep track of errors thrown in the below go routines
	// Using go routines sped up the logic from ~8 seconds to ~5 seconds as we wait for the generation to complete
	var eg errgroup.Group

	if claGroup.ProjectICLAEnabled {
		// Invoke the go routine - any errors will be handled below
		eg.Go(func() error {
			log.WithFields(f).Debugf("Creating PDF for %s", claTypeICLA)
			ioReader, iclaErr := s.docRaptorClient.CreatePDF(iclaTemplateHTML, claTypeICLA)
			if iclaErr != nil {
				log.WithFields(f).WithError(iclaErr).Warn("Problem generating ICLA template via docraptor client - returning empty template PDFs")
				return err
			}
			defer func() {
				closeErr := ioReader.Close()
				if closeErr != nil {
					log.WithFields(f).WithError(closeErr).Warn("error closing ICLA PDF")
				}
			}()
			iclaFileName := s.generateTemplateS3FilePath(claGroupID, claTypeICLA)
			iclaFileURL, err = s.SaveTemplateToS3(bucket, iclaFileName, ioReader)
			if err != nil {
				log.WithFields(f).WithError(err).Warnf("Problem uploading ICLA PDF: %s to s3 - returning empty template PDFs", iclaFileName)
				return err
			}

			template.IclaHTMLBody = iclaTemplateHTML
			return nil
		})
	}

	if claGroup.ProjectCCLAEnabled {
		// Invoke the go routine - any errors will be handled below
		eg.Go(func() error {
			log.WithFields(f).Debugf("Creating PDF for %s", claTypeCCLA)
			ioReader, cclaErr := s.docRaptorClient.CreatePDF(cclaTemplateHTML, claTypeCCLA)
			if cclaErr != nil {
				log.WithFields(f).WithError(cclaErr).Warn("Problem generating CCLA template via docraptor client - returning empty template PDFs")
				return err
			}
			defer func() {
				closeErr := ioReader.Close()
				if closeErr != nil {
					log.WithFields(f).WithError(closeErr).Warn("error closing CCLA PDF")
				}
			}()
			cclaFileName := s.generateTemplateS3FilePath(claGroupID, claTypeCCLA)
			cclaFileURL, err = s.SaveTemplateToS3(bucket, cclaFileName, ioReader)
			if err != nil {
				log.WithFields(f).Warnf("Problem uploading CCLA PDF: %s to s3, error: %v - returning empty template PDFs", cclaFileName, err)
				return err
			}

			template.CclaHTMLBody = cclaTemplateHTML
			return nil
		})
	}

	// Wait for the go routines to finish
	log.WithFields(f).Debug("Waiting for PDF generation to complete...")
	if pdfErr := eg.Wait(); pdfErr != nil {
		return models.TemplatePdfs{}, pdfErr
	}

	if claGroup.ProjectICLAEnabled && claGroup.ProjectCCLAEnabled {
		pdfUrls = models.TemplatePdfs{
			IndividualPDFURL: iclaFileURL,
			CorporatePDFURL:  cclaFileURL,
		}
	} else if claGroup.ProjectCCLAEnabled {
		pdfUrls = models.TemplatePdfs{
			CorporatePDFURL: cclaFileURL,
		}
	} else if claGroup.ProjectICLAEnabled {
		pdfUrls = models.TemplatePdfs{
			IndividualPDFURL: iclaFileURL,
		}
	}

	// Save Template to DynamoDB
	f["cclaEnabled"] = claGroup.ProjectCCLAEnabled
	f["iclaEnabled"] = claGroup.ProjectICLAEnabled
	log.WithFields(f).Debug("updating templates for the cla group")
	err = s.templateRepo.UpdateDynamoContractGroupTemplates(ctx, claGroupID, template, pdfUrls, claGroup.ProjectCCLAEnabled, claGroup.ProjectICLAEnabled)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("Problem updating the database with ICLA/CCLA new PDF details, error: %v - returning empty template PDFs", err)
		return models.TemplatePdfs{}, err
	}

	return pdfUrls, nil
}

// GetCLATemplatePreview returns a preview of the specified CLA Group and CLA type
func (s Service) GetCLATemplatePreview(ctx context.Context, claGroupID, claType string, watermark bool) ([]byte, error) {
	f := logrus.Fields{
		"functionName":   "v1.template.service.GetCLATemplatePreview",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
		"claGroupID":     claGroupID,
		"claType":        claType,
		"watermark":      watermark,
	}

	// Verify claGroupID matches an existing CLA Group
	claGroup, err := s.templateRepo.GetCLAGroup(claGroupID)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("unable to fetch CLA group by id: %s - returning empty PDF", claGroupID)
		return nil, err
	}

	var claGroupDocuments []models.ClaGroupDocument

	switch claType {
	case claTypeICLA:
		if !claGroup.ProjectICLAEnabled {
			err = fmt.Errorf("icla required for the group id : %s, but not enabled", claGroupID)
			log.WithFields(f).WithError(err)
			return nil, err
		}

	case claTypeCCLA:
		if !claGroup.ProjectCCLAEnabled {
			err = fmt.Errorf("ccla required for the group id : %s, but not enabled", claGroupID)
			log.WithFields(f).WithError(err)
			return nil, err
		}

	default:
		err = fmt.Errorf("not supported cla type provided : %s", claType)
		log.WithFields(f).WithError(err)
		return nil, err
	}

	claGroupDocuments, err = s.templateRepo.GetCLADocuments(claGroupID, claType)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("fetching icla document failed for claGroupID : %s", claGroupID)
		return nil, err
	}

	// process the documents and try to fetch the document from s3
	if len(claGroupDocuments) == 0 {
		err = fmt.Errorf("no documents found in groupID : %s", claGroupID)
		log.WithFields(f).WithError(err)
		return nil, err
	}

	doc := getLatestDocument(ctx, claGroupDocuments)
	pdfS3URL := doc.DocumentS3URL
	if pdfS3URL == "" {
		err = fmt.Errorf("s3 url is empty for groupID : %s and document %s", claGroupID, doc.DocumentFileID)
		log.WithFields(f).WithError(err)
		return nil, err
	}

	// Convert:
	//   https://cla-signature-files-dev.s3.amazonaws.com/contract-group/66b97366-a298-4625-965e-0c292c39f9a2/template/ccla-2020-09-25T22-37-51Z.pdf
	// to:
	//   contract-group/66b97366-a298-4625-965e-0c292c39f9a2/template/ccla-2020-09-25T22-37-51Z.pdf
	fileName, urlErr := utils.GetPathFromURL(pdfS3URL)
	if urlErr != nil {
		log.WithFields(f).WithError(urlErr).Warnf("problem obtaining path from URL: %s", pdfS3URL)
		return nil, err
	}

	// Strip any leading slashes...
	fileName = strings.TrimLeft(fileName, "/")

	// fetch the document from s3 at this stage
	b, err := utils.DownloadFromS3(fileName)
	if err != nil {
		log.WithFields(f).WithError(err).Warnf("problem downloading document from s3 using filename: %s", fileName)
		return nil, err
	}

	// do the watermarking here if enabled
	if watermark {
		b, err = utils.WatermarkPdf(b, "Not for Execution")
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem generating watermark pdf")
			return nil, err
		}
	}

	return b, nil
}

func getLatestDocument(ctx context.Context, documents []models.ClaGroupDocument) *models.ClaGroupDocument {
	f := logrus.Fields{
		"functionName":   "v1.template.service.getLatestDocument",
		utils.XREQUESTID: ctx.Value(utils.XREQUESTID),
	}
	var latestDocument *models.ClaGroupDocument
	var latestMajorVersion = 0
	var latestMinorVersion = 0
	var latestDateTime time.Time
	for _, currentDocument := range documents {
		if latestDocument == nil {
			latestDocument = &currentDocument // nolint
			// Grab and save the major version
			major, convertErr := strconv.Atoi(latestDocument.DocumentMajorVersion)
			if convertErr != nil {
				log.WithFields(f).WithError(convertErr).Warnf("problem converting document major version to int: %s", latestDocument.DocumentMajorVersion)
				major = 0
			}
			latestMajorVersion = major

			// Grab and save the major version
			minor, convertErr := strconv.Atoi(latestDocument.DocumentMinorVersion)
			if convertErr != nil {
				log.WithFields(f).WithError(convertErr).Warnf("problem converting document minor version to int: %s", latestDocument.DocumentMinorVersion)
				minor = 0
			}
			latestMinorVersion = minor

			dateTime, dateTimeErr := utils.ParseDateTime(latestDocument.DocumentCreationDate)
			if dateTimeErr != nil {
				log.WithFields(f).WithError(dateTimeErr).Warnf("problem converting document creation date to time object: %s", latestDocument.DocumentCreationDate)
			}
			latestDateTime = dateTime

			continue
		}

		// Grab and save the major version
		major, convertErr := strconv.Atoi(currentDocument.DocumentMajorVersion)
		if convertErr != nil {
			log.WithFields(f).WithError(convertErr).Warnf("problem converting document major version to int: %s", currentDocument.DocumentMajorVersion)
			major = 0
		}

		// Grab and save the major version
		minor, convertErr := strconv.Atoi(currentDocument.DocumentMinorVersion)
		if convertErr != nil {
			log.WithFields(f).WithError(convertErr).Warnf("problem converting document minor version to int: %s", currentDocument.DocumentMinorVersion)
			minor = 0
		}

		dateTime, dateTimeErr := utils.ParseDateTime(currentDocument.DocumentCreationDate)
		if dateTimeErr != nil {
			log.WithFields(f).WithError(dateTimeErr).Warnf("problem converting document creation date to time object: %s", currentDocument.DocumentCreationDate)
		}

		if major > latestMajorVersion {
			latestDocument = &currentDocument // nolint
			continue
		}

		if minor > latestMinorVersion {
			latestDocument = &currentDocument // nolint
			continue
		}

		if dateTime.After(latestDateTime) {
			latestDocument = &currentDocument // nolint
			continue
		}
	}

	return latestDocument
}

// InjectProjectInformationIntoTemplate service function
func (s Service) InjectProjectInformationIntoTemplate(template models.Template, metaFields []*models.MetaField) (string, string, error) {
	f := logrus.Fields{
		"functionName": "v1.template.service.InjectProjectInformationIntoTemplate",
		"templateName": template.Name,
		"templateID":   template.ID,
	}
	lookupMap := map[string]models.MetaField{}
	for _, field := range template.MetaFields {
		lookupMap[field.Name] = *field
	}

	metaFieldsMap := map[string]string{}
	for _, metaField := range metaFields {

		val, ok := lookupMap[metaField.Name]
		if !ok {
			continue
		}

		if val.Name == metaField.Name && val.TemplateVariable == metaField.TemplateVariable {
			if metaField.Value == "" {
				return "", "", fmt.Errorf("bad request: template field value of variable %s cannot be empty", metaField.TemplateVariable)
			}
			metaFieldsMap[metaField.TemplateVariable] = metaField.Value
		}
	}
	if len(template.MetaFields) != len(metaFieldsMap) {
		return "", "", errors.New("bad request: required fields for template were not found")
	}

	log.WithFields(f).Debugf("Rendering ICLA body for template: %s with id: %s", template.Name, template.ID)
	iclaTemplateHTML, err := raymond.Render(template.IclaHTMLBody, metaFieldsMap)
	if err != nil {
		return "", "", err
	}

	log.WithFields(f).Debugf("Rendering CCLA body for template: %s with id: %s", template.Name, template.ID)
	cclaTemplateHTML, err := raymond.Render(template.CclaHTMLBody, metaFieldsMap)
	if err != nil {
		return "", "", err
	}

	return iclaTemplateHTML, cclaTemplateHTML, nil
}

// generateTemplateS3FilePath helper function to generate a suitable s3 path and filename for the template
func (s Service) generateTemplateS3FilePath(claGroupID, claType string) string {
	fileNameTemplate := "contract-group/%s/template/%s"
	var ext string
	switch claType {
	case claTypeICLA:
		// Format would be, for example: icla-2020-09-25T22-32-59Z.pdf
		ext = fmt.Sprintf("icla-%s.pdf", strings.ReplaceAll(utils.CurrentSimpleDateTimeString(), ":", "-"))
	case claTypeCCLA:
		ext = fmt.Sprintf("ccla-%s.pdf", strings.ReplaceAll(utils.CurrentSimpleDateTimeString(), ":", "-"))
	default:
		return ""
	}
	fileName := fmt.Sprintf(fileNameTemplate, claGroupID, ext)
	return fileName
}

// SaveTemplateToS3 uploads the specified template contents to S3 storage
func (s Service) SaveTemplateToS3(bucket, filepath string, template io.ReadCloser) (string, error) {
	f := logrus.Fields{
		"functionName": "v1.template.service.SaveTemplateToS3",
		"bucket":       bucket,
		"filepath":     filepath,
	}
	defer func() {
		closeErr := template.Close()
		if closeErr != nil {
			log.WithFields(f).WithError(closeErr).Warn("error closing template")
		}
	}()

	// Upload the file to S3.
	result, err := s.s3Client.Upload(&s3manager.UploadInput{
		Bucket:      aws.String(bucket),
		Key:         aws.String(filepath),
		Body:        template,
		ACL:         aws.String("public-read"),
		ContentType: aws.String("application/pdf"),
	})
	if err != nil {
		return "", fmt.Errorf("failed to upload file to S3 Bucket: %s / %s, %v", bucket, filepath, err)
	}

	return result.Location, nil
}

// CLAGroupTemplateExists return true if the specified template ID exists, false otherwise
func (s Service) CLAGroupTemplateExists(ctx context.Context, templateID string) bool {
	return s.templateRepo.CLAGroupTemplateExists(ctx, templateID)
}
