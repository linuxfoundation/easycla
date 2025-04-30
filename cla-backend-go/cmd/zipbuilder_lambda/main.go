// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"context"
	"os"

	"github.com/communitybridge/easycla/cla-backend-go/utils"

	"github.com/communitybridge/easycla/cla-backend-go/v2/signatures"

	"github.com/aws/aws-lambda-go/lambda"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	log "github.com/communitybridge/easycla/cla-backend-go/logging"
)

var (
	// version the application version
	version string

	// build/Commit the application build number
	commit string

	// branch the build branch
	branch string

	// build date
	buildDate string
)

// BuildZipEvent is argument to zipbuilder
type BuildZipEvent struct {
	ClaGroupID    string `json:"cla_group_id"`
	SignatureType string `json:"signature_type"`
	FileType      string `json:"file_type"`
}

var zipBuilder signatures.ZipBuilder

func init() {
	var awsSession = session.Must(session.NewSession(&aws.Config{}))
	stage := os.Getenv("STAGE")
	if stage == "" {
		log.Fatal("stage not set")
	}
	log.Infof("STAGE : %s", stage)
	signaturesFileBucket := os.Getenv("CLA_SIGNATURE_FILES_BUCKET")
	if signaturesFileBucket == "" {
		log.Fatal("CLA_SIGNATURE_FILES_BUCKET is not set in environment")
	}
	log.Infof("CLA_SIGNATURE_FILES_BUCKET : %s", signaturesFileBucket)
	zipBuilder = signatures.NewZipBuilder(awsSession, signaturesFileBucket)
}

func handler(ctx context.Context, event BuildZipEvent) error {
	var err error
	log.WithField("event", event).Debug("zip builder called")
	switch event.SignatureType {
	case utils.ClaTypeICLA:
		if event.FileType == utils.FileTypePDF {
			err = zipBuilder.BuildICLAPDFZip(event.ClaGroupID)
		} else if event.FileType == utils.FileTypeCSV {
			err = zipBuilder.BuildICLACSVZip(event.ClaGroupID)
		} else {
			log.WithField("event", event).Warn("Invalid event")
		}
	case utils.ClaTypeCCLA:
		if event.FileType == utils.FileTypePDF {
			err = zipBuilder.BuildCCLAPDFZip(event.ClaGroupID)
		} else if event.FileType == utils.FileTypeCSV {
			err = zipBuilder.BuildCCLACSVZip(event.ClaGroupID)
		} else {
			log.WithField("event", event).Warn("Invalid event")
		}
	case utils.ClaTypeECLA:
		if event.FileType == utils.FileTypeCSV {
			err = zipBuilder.BuildECLACSVZip(event.ClaGroupID)
		} else {
			log.WithField("event", event).Warn("Invalid event")
		}
	default:
		log.WithField("event", event).Warn("Invalid event")
	}
	if err != nil {
		log.WithField("args", event).Error("failed to build zip", err)
	}
	return err
}

func printBuildInfo() {
	log.Infof("Version                 : %s", version)
	log.Infof("Git commit hash         : %s", commit)
	log.Infof("Branch                  : %s", branch)
	log.Infof("Build date              : %s", buildDate)
}

func main() {
	log.Info("Lambda server starting...")
	printBuildInfo()
	if os.Getenv("LOCAL_MODE") == "true" {
		if len(os.Args) != 4 {
			log.Fatal("invalid number of args. first arg should be icla or ccla, 2nd should be pdf or csv and 3rd arg should be cla_group_id")
		}
		err := handler(utils.NewContext(), BuildZipEvent{SignatureType: os.Args[1], FileType: os.Args[2], ClaGroupID: os.Args[3]})
		if err != nil {
			log.Fatal(err)
		}
	} else {
		lambda.Start(handler)
	}
	log.Infof("Lambda shutting down...")
}
