// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"encoding/csv"
	"encoding/xml"
	"strings"
	"sync"

	"flag"

	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"

	"github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/config"
	"github.com/linuxfoundation/easycla/cla-backend-go/github_organizations"

	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	"github.com/linuxfoundation/easycla/cla-backend-go/users"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/sign"

	"github.com/linuxfoundation/easycla/cla-backend-go/utils"

	"github.com/sirupsen/logrus"
)

var stage string
var signatureRepo signatures.SignatureRepository
var awsSession = session.Must(session.NewSession(&aws.Config{}))
var companyRepo company.IRepository
var usersRepo users.UserRepository

var signService sign.Service
var githubOrgService github_organizations.Service
var report []ReportData
var failed int = 0
var success int = 0

func init() {
	stage = os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("STAGE environment variable not set")
	}

	companyRepo = company.NewRepository(awsSession, stage)
	usersRepo = users.NewRepository(awsSession, stage)
	signatureRepo = signatures.NewRepository(awsSession, stage, companyRepo, usersRepo, nil, nil, nil, nil, nil)
	githubOrgService = github_organizations.Service{}
	configFile, err := config.LoadConfig("", awsSession, stage)
	if err != nil {
		log.Fatal(err)
	}
	signService = sign.NewService("", "", companyRepo, nil, nil, nil, nil, configFile.DocuSignPrivateKey, nil, nil, nil, nil, githubOrgService, nil, "", "", nil, nil, nil, nil, nil)
	// projectRepo = repository.NewRepository(awsSession, stage, nil, nil, nil)
	utils.SetS3Storage(awsSession, configFile.SignatureFilesBucket)
}

const (
	// Approved Flag
	Approved = true
	// Signed Flag
	Signed = true

	Failed           = "failed"
	Success          = "success"
	DocumentUploaded = "Document uploaded successfully"
)

type ReportData struct {
	SignatureID   string
	ProjectID     string
	ReferenceID   string
	ReferenceName string
	EnvelopeID    string
	DocumentID    string
	Comment       string
	Status        string
}

type APIErrorResponse struct {
	ErrorCode string `json:"errorCode"`
	Message   string `json:"message"`
}

func main() { // nolint
	ctx := context.Background()
	f := logrus.Fields{
		"functionName": "main",
	}
	// var toUpdate []*signatures.ItemSignature

	dryRun := flag.Bool("dry-run", false, "dry run mode")
	folder := flag.String("folder", "", "folder to upload the s3 documents")
	meta := flag.String("meta", "", "meta data to upload the s3 documents")

	flag.Parse()

	// Fetch all the signatures from 2024-02-01T00:00:00.000Z
	startDate := "2024-02-01T00:00:00.000Z"

	if dryRun != nil && *dryRun {
		log.WithFields(f).Debug("dry-run mode enabled")
	}

	if folder != nil && *folder != "" && meta != nil && *meta != "" {
		log.WithFields(f).Debugf("folder: %s, meta: %s", *folder, *meta)
		// var metaMap map[string]string

		// Read csv file
		file, err := os.Open(*meta)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem opening meta file")
			return
		}

		reader := csv.NewReader(file)
		records, err := reader.ReadAll()

		count := len(records)
		log.WithFields(f).Debugf("processing %d records", count)

		passed := 0

		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem reading meta file")
			return
		}

		var wg sync.WaitGroup

		// Limit the number of concurrent uploads
		semaphore := make(chan struct{}, 5)

		for _, record := range records {
			wg.Add(1)
			semaphore <- struct{}{}
			go func(record []string) {
				defer wg.Done()
				defer func() { <-semaphore }()
				fileName := record[0]
				envelopeID := record[1]
				signatureID := record[2]
				projectID := record[3]
				referenceID := record[4]
				log.WithFields(f).Debugf("uploading file: %s, envelopeID: %s, signatureID: %s, projectID: %s, referenceID: %s", fileName, envelopeID, signatureID, projectID, referenceID)
				// Upload the file
				file, err := os.Open(*folder + "/" + fileName) // nolint
				if err != nil {
					log.WithFields(f).WithError(err).Warn("problem opening file")
					failed++
					return
				}

				if dryRun != nil && *dryRun {
					log.WithFields(f).Debugf("dry-run mode enabled, skipping file upload: %s", fileName)
					return
				}

				// Upload the document
				log.WithFields(f).Debugf("uploading document for signature...: %s", signatureID)

				err = utils.UploadFileToS3(file, projectID, utils.ClaTypeICLA, referenceID, signatureID)

				if err != nil {
					log.WithFields(f).WithError(err).Warn("problem uploading file")
					failed++
					return
				}
				passed++

				log.WithFields(f).Debugf("document uploaded for signature: %s", signatureID)

			}(record)
		}

		wg.Wait()

		log.WithFields(f).Debug("completed processing files")

		log.WithFields(f).Debugf("total: %d, passed: %d, failed: %d", count, passed, failed)
		return
	}

	iclaSignatures, err := signatureRepo.GetICLAByDate(ctx, startDate)
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem fetching ICLA signatures")
		return
	}

	log.WithFields(f).Debugf("processing %d ICLA signatures", len(iclaSignatures))
	toUpload := make([]signatures.ItemSignature, 0)

	var wg sync.WaitGroup
	semaphore := make(chan struct{}, 20)

	for _, icla := range iclaSignatures {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(sig signatures.ItemSignature) {
			defer wg.Done()
			defer func() { <-semaphore }()
			key := strings.Join([]string{"contract-group", sig.SignatureProjectID, utils.ClaTypeICLA, sig.SignatureReferenceID, sig.SignatureID}, "/") + ".pdf"
			fileExists, fileErr := utils.DocumentExists(key)
			if fileErr != nil {
				log.WithFields(f).WithError(fileErr).Debugf("unable to check s3 key : %s", key)
				return
			}
			if !fileExists {
				log.WithFields(f).Debugf("document is not uploaded for key: %s", key)
				toUpload = append(toUpload, sig)
			} else {
				log.WithFields(f).Debugf("key: %s exists", key)
			}
		}(icla)

	}

	log.WithFields(f).Debugf("checking icla signatures from :%s", startDate)
	wg.Wait()
	log.WithFields(f).Debugf("To upload %d icla signatures: ", len(toUpload))

	// Upload the documents
	for _, icla := range toUpload {
		wg.Add(1)
		semaphore <- struct{}{}
		go func(sig signatures.ItemSignature) {
			defer wg.Done()

			var documentID string

			reportData := ReportData{
				SignatureID:   sig.SignatureID,
				ProjectID:     sig.SignatureProjectID,
				ReferenceID:   sig.SignatureReferenceID,
				ReferenceName: sig.SignatureReferenceName,
				EnvelopeID:    sig.SignatureEnvelopeID,
			}

			// get the document id
			var info sign.DocuSignEnvelopeInformation

			if sig.UserDocusignRawXML == "" {
				log.WithFields(f).Debugf("no raw xml found for signature: %s", sig.SignatureID)
				reportData.Comment = "No raw xml found"
				// Fetch documentID
				documents, docErr := signService.GetEnvelopeDocuments(ctx, sig.SignatureEnvelopeID)
				if docErr != nil {
					log.WithFields(f).WithError(err).Debugf("unable to get documents for signature: %s", sig.SignatureID)
					reportData.Comment = docErr.Error()
					reportData.Status = Failed
					report = append(report, reportData)
					failed++
					return
				}
				if len(documents) == 0 {
					log.WithFields(f).Debugf("no documents found for signature: %s", sig.SignatureID)
					reportData.Comment = "No documents found"
					reportData.Status = Failed
					report = append(report, reportData)
					failed++
					return
				}
				documentID = documents[0].DocumentId
				log.WithFields(f).Debugf("document id fetched from docusign: %s", documentID)
			} else {
				err = xml.Unmarshal([]byte(sig.UserDocusignRawXML), &info)
				if err != nil {
					log.WithFields(f).WithError(err).Debugf("unable to unmarshal xml for signature: %s", sig.SignatureID)
					reportData.Comment = err.Error()
					reportData.Status = Failed
					report = append(report, reportData)
					failed++
					return
				}
				documentID = info.EnvelopeStatus.DocumentStatuses[0].ID
			}

			log.WithFields(f).Debugf("document id: %s", documentID)
			reportData.DocumentID = documentID
			envelopeID := sig.SignatureEnvelopeID
			log.WithFields(f).Debugf("envelope id: %s", envelopeID)

			if documentID == "" {
				log.WithFields(f).Debugf("no document id found for signature: %s", sig.SignatureID)
				reportData.Comment = "No document id found"
				reportData.Status = Failed
				report = append(report, reportData)
				failed++
				return
			}

			// get the document
			document, docErr := signService.GetSignedDocument(ctx, envelopeID, documentID)
			if docErr != nil {
				log.WithFields(f).WithError(docErr).Debugf("unable to get document for signature: %s", sig.SignatureID)
				reportData.Comment = docErr.Error()
				reportData.Status = Failed
				report = append(report, reportData)
				failed++
				return
			}
			// upload the document
			if dryRun != nil && *dryRun {
				log.WithFields(f).Debugf("dry-run mode enabled, skipping document upload for signature: %s", sig.SignatureID)
				log.WithFields(f).Debugf("document uploaded for signature: %s", sig.SignatureID)
				reportData.Comment = DocumentUploaded
				reportData.Status = Success
				report = append(report, reportData)
				return
			}

			log.WithFields(f).Debugf("uploading document for signature...: %s", sig.SignatureID)
			err = utils.UploadToS3(document, sig.SignatureProjectID, utils.ClaTypeICLA, sig.SignatureReferenceID, sig.SignatureID)
			if err != nil {
				log.WithFields(f).WithError(err).Debugf("unable to upload document for signature: %s", sig.SignatureID)
				reportData.Comment = err.Error()
				reportData.Status = Failed
				failed++
				report = append(report, reportData)
				return
			}

			log.WithFields(f).Debugf("document uploaded for signature: %s", sig.SignatureID)
			reportData.Comment = DocumentUploaded
			reportData.Status = Success
			success++

			report = append(report, reportData)

			// release the semaphore
			<-semaphore

		}(icla)
	}

	wg.Wait()

	log.WithFields(f).Debug("completed processing ICLA signatures")

	file, err := os.Create("s3_upload_report.csv")
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem creating report file")
		return
	}

	writer := csv.NewWriter(file)
	defer writer.Flush()

	err = writer.Write([]string{"SignatureID", "ProjectID", "ReferenceID", "ReferenceName", "EnvelopeID", "DocumentID", "Comment", "Status"})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("problem writing header to report file")
		return
	}

	for _, data := range report {
		// writer.Write([]string{data.SignatureID, data.ProjectID, data.ReferenceID, data.ReferenceName, data.EnvelopeID, data.DocumentID, data.Comment})
		record := []string{data.SignatureID, data.ProjectID, data.ReferenceID, data.ReferenceName, data.EnvelopeID, data.DocumentID, data.Comment, data.Status}
		err = writer.Write(record)
		if err != nil {
			log.WithFields(f).WithError(err).Warn("problem writing record to report file")
		}
	}

	log.WithFields(f).Debugf("report generated successfully, total: %d, success: %d, failed: %d", len(report), success, failed)

	log.WithFields(f).Debug("report generated successfully")
}
