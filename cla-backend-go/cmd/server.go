// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package cmd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"

	"github.com/linuxfoundation/easycla/cla-backend-go/project/repository"
	"github.com/linuxfoundation/easycla/cla-backend-go/project/service"

	gitlab_activity "github.com/linuxfoundation/easycla/cla-backend-go/v2/gitlab-activity"

	"github.com/go-openapi/strfmt"

	"github.com/linuxfoundation/easycla/cla-backend-go/v2/gitlab_organizations"

	gitlab "github.com/linuxfoundation/easycla/cla-backend-go/gitlab_api"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/gitlab_sign"

	"github.com/linuxfoundation/easycla/cla-backend-go/emails"

	"github.com/linuxfoundation/easycla/cla-backend-go/v2/dynamo_events"
	v2GithubActivity "github.com/linuxfoundation/easycla/cla-backend-go/v2/github_activity"

	"github.com/gofrs/uuid"

	openapi_runtime "github.com/go-openapi/runtime"
	"github.com/linuxfoundation/easycla/cla-backend-go/approval_list"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/cla_groups"

	"github.com/linuxfoundation/easycla/cla-backend-go/projects_cla_groups"

	"github.com/linuxfoundation/easycla/cla-backend-go/v2/sign"

	"github.com/linuxfoundation/easycla/cla-backend-go/cla_manager"
	project_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/project-service"
	user_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/user-service"

	acs_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/acs-service"
	organization_service "github.com/linuxfoundation/easycla/cla-backend-go/v2/organization-service"

	"github.com/linuxfoundation/easycla/cla-backend-go/github_organizations"
	v2GithubOrganizations "github.com/linuxfoundation/easycla/cla-backend-go/v2/github_organizations"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/metrics"

	"github.com/linuxfoundation/easycla/cla-backend-go/gerrits"
	v2Gerrits "github.com/linuxfoundation/easycla/cla-backend-go/v2/gerrits"

	"github.com/aws/aws-sdk-go/service/dynamodb"

	lfxAuth "github.com/LF-Engineering/lfx-kit/auth"
	"github.com/linuxfoundation/easycla/cla-backend-go/docs"
	v1Repositories "github.com/linuxfoundation/easycla/cla-backend-go/repositories"
	"github.com/linuxfoundation/easycla/cla-backend-go/utils"
	v2Docs "github.com/linuxfoundation/easycla/cla-backend-go/v2/docs"
	v2Events "github.com/linuxfoundation/easycla/cla-backend-go/v2/events"
	v2Metrics "github.com/linuxfoundation/easycla/cla-backend-go/v2/metrics"
	v2Repositories "github.com/linuxfoundation/easycla/cla-backend-go/v2/repositories"
	v2Version "github.com/linuxfoundation/easycla/cla-backend-go/v2/version"
	"github.com/linuxfoundation/easycla/cla-backend-go/version"

	"github.com/linuxfoundation/easycla/cla-backend-go/events"

	"github.com/linuxfoundation/easycla/cla-backend-go/project"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/approvals"
	v2Project "github.com/linuxfoundation/easycla/cla-backend-go/v2/project"

	"github.com/linuxfoundation/easycla/cla-backend-go/users"

	"github.com/linuxfoundation/easycla/cla-backend-go/signatures"
	v2Signatures "github.com/linuxfoundation/easycla/cla-backend-go/v2/signatures"

	ini "github.com/linuxfoundation/easycla/cla-backend-go/init"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"

	"github.com/linuxfoundation/easycla/cla-backend-go/auth"
	v1Company "github.com/linuxfoundation/easycla/cla-backend-go/company"
	"github.com/linuxfoundation/easycla/cla-backend-go/docraptor"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/models"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi"
	"github.com/linuxfoundation/easycla/cla-backend-go/gen/v1/restapi/operations"
	v2RestAPI "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi"
	v2Ops "github.com/linuxfoundation/easycla/cla-backend-go/gen/v2/restapi/operations"
	"github.com/linuxfoundation/easycla/cla-backend-go/github"
	"github.com/linuxfoundation/easycla/cla-backend-go/health"
	"github.com/linuxfoundation/easycla/cla-backend-go/template"
	"github.com/linuxfoundation/easycla/cla-backend-go/user"
	v2ClaManager "github.com/linuxfoundation/easycla/cla-backend-go/v2/cla_manager"
	v2Company "github.com/linuxfoundation/easycla/cla-backend-go/v2/company"
	v2CurrentUser "github.com/linuxfoundation/easycla/cla-backend-go/v2/current_user"
	v2Health "github.com/linuxfoundation/easycla/cla-backend-go/v2/health"
	"github.com/linuxfoundation/easycla/cla-backend-go/v2/store"
	v2Template "github.com/linuxfoundation/easycla/cla-backend-go/v2/template"

	"github.com/go-openapi/loads"
	"github.com/rs/cors"
	"github.com/savaki/dynastore"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	// Version is the application version - either a git SHA or tag value
	Version string

	// Commit is the application commit hash
	Commit string

	// Branch the build branch
	Branch string

	// BuildDate is the date of the build
	BuildDate string
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "server",
	Short: "Run the backend server",
	Long:  `Run the backend server which listens for http requests over a given port.`,
	Run:   runServer,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

type combinedRepo struct {
	users.UserRepository
	v1Company.IRepository
	repository.ProjectRepository
	projects_cla_groups.Repository
}

// server function called by environment specific server functions
func server(localMode bool) http.Handler {
	f := logrus.Fields{
		"functionName": "cmd.server",
		"localMode":    localMode,
	}

	host, err := os.Hostname()
	if err != nil {
		log.WithFields(f).WithError(err).Fatalf("unable to get hostname. Error: %v", err)
	}

	var githubOrgValidation = true // default is true/enabled
	githubOrgValidationString := viper.GetString("GH_ORG_VALIDATION")
	if githubOrgValidationString != "" {
		githubOrgValidation, err = strconv.ParseBool(githubOrgValidationString)
		if err != nil {
			log.WithFields(f).WithError(err).Fatal("GH_ORG_VALIDATION value must be a boolean string")
		}
	}

	var companyUserValidation = true // default is true/enabled
	companyUserValidationString := viper.GetString("COMPANY_USER_VALIDATION")
	if companyUserValidationString != "" {
		companyUserValidation, err = strconv.ParseBool(companyUserValidationString)
		if err != nil {
			log.WithFields(f).WithError(err).Fatal("COMPANY_USER_VALIDATION value must be a boolean string")
		}
	}

	stage := viper.GetString("STAGE")
	dynamodbRegion := ini.GetProperty("DYNAMODB_AWS_REGION")

	log.WithFields(f).Infof("Service %s starting...", ini.ServiceName)

	if log.IsTextLogFormat() {
		// Show the version and build info
		log.Infof("Name                    : %s", ini.ServiceName)
		log.Infof("Version                 : %s", Version)
		log.Infof("Git commit hash         : %s", Commit)
		log.Infof("Branch                  : %s", Branch)
		log.Infof("Build date              : %s", BuildDate)
		log.Infof("Golang OS               : %s", runtime.GOOS)
		log.Infof("Golang Arch             : %s", runtime.GOARCH)
		log.Infof("DYANAMODB_AWS_REGION    : %s", dynamodbRegion)
		log.Infof("GH_ORG_VALIDATION       : %t", githubOrgValidation)
		log.Infof("COMPANY_USER_VALIDATION : %t", companyUserValidation)
		log.Infof("STAGE                   : %s", stage)
		log.Infof("Service Host            : %s", host)
		log.Infof("Service Port            : %d", *portFlag)
	} else {
		f["serviceName"] = ini.ServiceName
		f["version"] = Version
		f["commit"] = Commit
		f["branch"] = Branch
		f["buildDate"] = BuildDate
		f["os"] = runtime.GOOS
		f["arch"] = runtime.GOARCH
		f["dynamoDBRegion"] = dynamodbRegion
		f["githubOrgValidation"] = githubOrgValidation
		f["companyUserValidation"] = companyUserValidation
		f["stage"] = stage
		f["serviceHost"] = host
		log.WithFields(f).Info("config")
	}

	awsSession, err := ini.GetAWSSession()
	if err != nil {
		log.WithFields(f).WithError(err).Panic("Unable to load AWS session")
	}

	configFile := ini.GetConfig()

	swaggerSpec, err := loads.Analyzed(restapi.SwaggerJSON, "")
	if err != nil {
		log.WithFields(f).WithError(err).Panic("Invalid swagger file for initializing EasyCLA v1")
	}

	v2SwaggerSpec, err := loads.Analyzed(v2RestAPI.SwaggerJSON, "")
	if err != nil {
		log.WithFields(f).WithError(err).Panic("Invalid swagger file for initializing EasyCLA v2")
	}

	api := operations.NewClaAPI(swaggerSpec)
	v2API := v2Ops.NewEasyclaAPI(v2SwaggerSpec)

	docraptorClient, err := docraptor.NewDocraptorClient(configFile.Docraptor.APIKey, configFile.Docraptor.TestMode)
	if err != nil {
		log.WithFields(f).WithError(err).Panic("unable to setup docraptor client")
	}

	// LG: to test with manual tokens
	customClaimUsername := os.Getenv("AUTH0_USERNAME_CLAIM_CLI")
	if customClaimUsername != "" {
		configFile.Auth0.UsernameClaim = customClaimUsername
	}
	nameClaimName := os.Getenv("AUTH0_NAME_CLAIM_CLI")
	if nameClaimName == "" {
		nameClaimName = "name"
	}
	emailClaimName := os.Getenv("AUTH0_EMAIL_CLAIM_CLI")
	if emailClaimName == "" {
		emailClaimName = "email"
	}
	authValidator, err := auth.NewAuthValidator(
		configFile.Auth0.Domain,
		configFile.Auth0.ClientID,
		configFile.Auth0.UsernameClaim,
		nameClaimName,
		emailClaimName,
		configFile.Auth0.Algorithm)
	if err != nil {
		logrus.Panic(err)
	}
	// initialize github
	github.Init(configFile.GitHub.AppID, configFile.GitHub.AppPrivateKey, configFile.GitHub.AccessToken)
	// initialize gitlab
	gitlabApp := gitlab.Init(configFile.Gitlab.AppClientID, configFile.Gitlab.AppClientSecret, configFile.Gitlab.AppPrivateKey)

	// Our backend repository handlers
	userRepo := user.NewDynamoRepository(awsSession, stage)
	usersRepo := users.NewRepository(awsSession, stage)
	gitV1Repository := v1Repositories.NewRepository(awsSession, stage)
	gitV2Repository := v2Repositories.NewRepository(awsSession, stage)
	gerritRepo := gerrits.NewRepository(awsSession, stage)
	templateRepo := template.NewRepository(awsSession, stage)
	approvalListRepo := approval_list.NewRepository(awsSession, stage)
	v1CompanyRepo := v1Company.NewRepository(awsSession, stage)
	eventsRepo := events.NewRepository(awsSession, stage)
	v1ProjectClaGroupRepo := projects_cla_groups.NewRepository(awsSession, stage)
	v1CLAGroupRepo := repository.NewRepository(awsSession, stage, gitV1Repository, gerritRepo, v1ProjectClaGroupRepo)
	metricsRepo := metrics.NewRepository(awsSession, stage, configFile.APIGatewayURL, v1ProjectClaGroupRepo)
	githubOrganizationsRepo := github_organizations.NewRepository(awsSession, stage)
	gitlabOrganizationRepo := gitlab_organizations.NewRepository(awsSession, stage)
	claManagerReqRepo := cla_manager.NewRepository(awsSession, stage)
	storeRepository := store.NewRepository(awsSession, stage)
	approvalsRepo := approvals.NewRepository(stage, awsSession, fmt.Sprintf("cla-%s-approvals", stage))

	// Our service layer handlers
	eventsService := events.NewService(eventsRepo, combinedRepo{
		usersRepo,
		v1CompanyRepo,
		v1CLAGroupRepo,
		v1ProjectClaGroupRepo,
	})

	gerritService := gerrits.NewService(gerritRepo)

	// Signature repository handler
	signaturesRepo := signatures.NewRepository(awsSession, stage, v1CompanyRepo, usersRepo, eventsService, gitV1Repository, githubOrganizationsRepo, gerritService, approvalsRepo)

	// Initialize the external platform services - these are external APIs that
	// we download the swagger specification, generate the models, and have
	//client helper functions
	user_service.InitClient(configFile.PlatformAPIGatewayURL, configFile.AcsAPIKey)
	project_service.InitClient(configFile.PlatformAPIGatewayURL)
	organization_service.InitClient(configFile.PlatformAPIGatewayURL, eventsService)
	acs_service.InitClient(configFile.PlatformAPIGatewayURL, configFile.AcsAPIKey)

	v1ProjectClaGroupService := projects_cla_groups.NewService(v1ProjectClaGroupRepo)
	usersService := users.NewService(usersRepo, eventsService)
	healthService := health.New(Version, Commit, Branch, BuildDate)
	templateService := template.NewService(stage, templateRepo, docraptorClient, awsSession)
	v1ProjectService := service.NewService(v1CLAGroupRepo, gitV1Repository, gerritRepo, v1ProjectClaGroupRepo, usersRepo)
	emailTemplateService := emails.NewEmailTemplateService(v1CLAGroupRepo, v1ProjectClaGroupRepo, v1ProjectService, configFile.CorporateConsoleV1URL, configFile.CorporateConsoleV2URL)
	emailService := emails.NewService(emailTemplateService, v1ProjectService)
	v2ProjectService := v2Project.NewService(v1ProjectService, v1CLAGroupRepo, v1ProjectClaGroupRepo)
	v1CompanyService := v1Company.NewService(v1CompanyRepo, configFile.CorporateConsoleV1URL, userRepo, usersService)
	v2CompanyService := v2Company.NewService(v1CompanyService, signaturesRepo, v1CLAGroupRepo, usersRepo, v1CompanyRepo, v1ProjectClaGroupRepo, eventsService)
	v2CurrentUserService := v2CurrentUser.NewService()

	v1RepositoriesService := v1Repositories.NewService(gitV1Repository, githubOrganizationsRepo, v1ProjectClaGroupRepo)
	v2RepositoriesService := v2Repositories.NewService(gitV1Repository, gitV2Repository, v1ProjectClaGroupRepo, githubOrganizationsRepo, gitlabOrganizationRepo, eventsService)
	githubOrganizationsService := github_organizations.NewService(githubOrganizationsRepo, gitV1Repository, v1ProjectClaGroupRepo)
	gitlabOrganizationsService := gitlab_organizations.NewService(gitlabOrganizationRepo, v2RepositoriesService, v1ProjectClaGroupRepo, storeRepository, usersService, signaturesRepo, v1CompanyRepo)
	v1SignaturesService := signatures.NewService(signaturesRepo, v1CompanyService, usersService, eventsService, githubOrgValidation, v1RepositoriesService, githubOrganizationsService, v1ProjectService, gitlabApp, configFile.ClaV1ApiURL, configFile.CLALandingPage, configFile.CLALogoURL)
	v2SignatureService := v2Signatures.NewService(awsSession, configFile.SignatureFilesBucket, v1ProjectService, v1CompanyService, v1SignaturesService, v1ProjectClaGroupRepo, signaturesRepo, usersService, approvalsRepo)
	v1ClaManagerService := cla_manager.NewService(claManagerReqRepo, v1ProjectClaGroupRepo, v1CompanyService, v1ProjectService, usersService, v1SignaturesService, eventsService, emailTemplateService, configFile.CorporateConsoleV1URL)
	v2ClaManagerService := v2ClaManager.NewService(emailTemplateService, v1CompanyService, v1ProjectService, v1ClaManagerService, usersService, v1RepositoriesService, v2CompanyService, eventsService, v1ProjectClaGroupRepo)
	v1ApprovalListService := approval_list.NewService(approvalListRepo, v1ProjectClaGroupRepo, v1ProjectService, usersRepo, v1CompanyRepo, v1CLAGroupRepo, signaturesRepo, emailTemplateService, configFile.CorporateConsoleV2URL, http.DefaultClient)
	authorizer := auth.NewAuthorizer(authValidator, userRepo)
	v2MetricsService := metrics.NewService(metricsRepo, v1ProjectClaGroupRepo)
	gitlabActivityService := gitlab_activity.NewService(gitV1Repository, gitV2Repository, usersRepo, signaturesRepo, v1ProjectClaGroupRepo, v1CompanyRepo, signaturesRepo, gitlabOrganizationsService)
	gitlabSignService := gitlab_sign.NewService(v2RepositoriesService, usersService, storeRepository, gitlabApp, gitlabOrganizationsService)
	v2GithubOrganizationsService := v2GithubOrganizations.NewService(githubOrganizationsRepo, gitV1Repository, v1ProjectClaGroupRepo, githubOrganizationsService)
	autoEnableService := dynamo_events.NewAutoEnableService(v1RepositoriesService, gitV1Repository, githubOrganizationsRepo, v1ProjectClaGroupRepo, v1ProjectService)
	v2GithubActivityService := v2GithubActivity.NewService(gitV1Repository, githubOrganizationsRepo, eventsService, autoEnableService, emailService)

	v2ClaGroupService := cla_groups.NewService(v1ProjectService, templateService, v1ProjectClaGroupRepo, v1ClaManagerService, v1SignaturesService, metricsRepo, gerritService, v1RepositoriesService, eventsService)
	v2SignService := sign.NewService(configFile.ClaAPIV4Base, configFile.ClaV1ApiURL, v1CompanyRepo, v1CLAGroupRepo, v1ProjectClaGroupRepo, v1CompanyService, v2ClaGroupService, configFile.DocuSignPrivateKey, usersService, v1SignaturesService, storeRepository, v1RepositoriesService, githubOrganizationsService, gitlabOrganizationsService, configFile.CLALandingPage, configFile.CLALogoURL, emailService, eventsService, gitlabActivityService, gitlabApp, gerritService)

	sessionStore, err := dynastore.New(dynastore.Path("/"), dynastore.HTTPOnly(), dynastore.TableName(configFile.SessionStoreTableName), dynastore.DynamoDB(dynamodb.New(awsSession)))
	if err != nil {
		log.WithFields(f).WithError(err).Panic("unable to create new Dynastore session")
	}
	utils.SetSnsEmailSender(awsSession, configFile.SNSEventTopicARN, configFile.SenderEmailAddress)
	utils.SetS3Storage(awsSession, configFile.SignatureFilesBucket)

	// Setup security handlers
	api.OauthSecurityAuth = authorizer.SecurityAuth
	v2API.LfAuthAuth = lfxAuth.SwaggerAuth

	// Setup our API handlers
	users.Configure(api, usersService, eventsService)
	project.Configure(api, v1ProjectService, eventsService, gerritService, v1RepositoriesService, v1SignaturesService)
	v2Project.Configure(v2API, v1ProjectService, v2ProjectService, eventsService, v1ProjectClaGroupService, v2RepositoriesService, gerritService)
	health.Configure(api, healthService)
	v2Health.Configure(v2API, healthService)
	template.Configure(api, templateService, eventsService)
	v2Template.Configure(v2API, templateService, v1ProjectClaGroupService, eventsService)
	github.Configure(api, configFile.GitHub.ClientID, configFile.GitHub.ClientSecret, configFile.GitHub.AccessToken, sessionStore)
	signatures.Configure(api, v1SignaturesService, sessionStore, eventsService)
	v2Signatures.Configure(v2API, v1ProjectService, v1CLAGroupRepo, v1CompanyService, v1SignaturesService, sessionStore, eventsService, v2SignatureService, v1ProjectClaGroupRepo)
	approval_list.Configure(api, v1ApprovalListService, sessionStore, v1SignaturesService, eventsService)
	v1Company.Configure(api, v1CompanyService, usersService, companyUserValidation, eventsService)
	docs.Configure(api)
	v2Docs.Configure(v2API)
	version.Configure(api, Version, Commit, Branch, BuildDate)
	v2Version.Configure(v2API, Version, Commit, Branch, BuildDate)
	events.Configure(api, eventsService)
	v2Events.Configure(v2API, eventsService, v1CompanyRepo, v1ProjectClaGroupRepo, v1ProjectService)
	v2Metrics.Configure(v2API, v2MetricsService, v1CompanyRepo)
	github_organizations.Configure(api, githubOrganizationsService, eventsService)
	v2GithubOrganizations.Configure(v2API, v2GithubOrganizationsService, eventsService)
	gitlab_organizations.Configure(v2API, gitlabOrganizationsService, eventsService, sessionStore, configFile.CLAContributorv2Base)
	gitlab_sign.Configure(v2API, gitlabSignService, eventsService, configFile.CLAContributorv2Base, sessionStore)
	gitlab_activity.Configure(v2API, gitlabActivityService, gitlabOrganizationsService, eventsService, gitlabApp, gitlabSignService, configFile.CLAContributorv2Base, sessionStore)
	v1Repositories.Configure(api, v1RepositoriesService, eventsService)
	v2Repositories.Configure(v2API, v2RepositoriesService, eventsService)
	gerrits.Configure(api, gerritService, v1ProjectService, eventsService)
	v2Gerrits.Configure(v2API, gerritService, v1ProjectService, eventsService, v1ProjectClaGroupRepo)
	v2Company.Configure(v2API, v2CompanyService, v1ProjectClaGroupRepo, configFile.LFXPortalURL, configFile.CorporateConsoleV1URL)
	v2CurrentUser.Configure(v2API, v2CurrentUserService)
	cla_manager.Configure(api, v1ClaManagerService, v1CompanyService, v1ProjectService, usersService, v1SignaturesService, eventsService, emailTemplateService)
	v2ClaManager.Configure(v2API, v2ClaManagerService, v1CompanyService, configFile.LFXPortalURL, configFile.CorporateConsoleV2URL, v1ProjectClaGroupRepo, userRepo)
	cla_groups.Configure(v2API, v2ClaGroupService, v1ProjectService, v1ProjectClaGroupRepo, eventsService)
	sign.Configure(v2API, v2SignService, usersService)
	v2GithubActivity.Configure(v2API, v2GithubActivityService)

	v2API.AddMiddlewareFor("POST", "/signed/individual/{installation_id}/{github_repository_id}/{change_request_id}", sign.DocusignMiddleware)
	v2API.AddMiddlewareFor("POST", "/signed/corporate/{project_id}/{company_id}", sign.CCLADocusignMiddleware)
	v2API.AddMiddlewareFor("POST", "/signed/gitlab/individual/{user_id}/{organization_id}/{gitlab_repository_id}/{merge_request_id}", sign.DocusignMiddleware)
	v2API.AddMiddlewareFor("POST", "/signed/gerrit/individual/{user_id}", sign.DocusignMiddleware)

	userCreaterMiddleware := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r = createUserFromRequest(authorizer, usersService, eventsService, r)
			next.ServeHTTP(w, r)
		})
	}

	// The middleware configuration is for the handler executors. These do not apply to the swagger.json document.
	// The middleware executes after routing but before authentication, binding and validation
	middlewareSetupfunc := func(handler http.Handler) http.Handler {
		return setRequestIDHandler(responseLoggingMiddleware(userCreaterMiddleware(handler)))
	}

	v2API.CsvProducer = openapi_runtime.ProducerFunc(func(w io.Writer, data interface{}) error {
		switch v := data.(type) {
		case []byte:
			_, err := w.Write(v)
			return err
		case []string:
			if len(v) == 0 {
				return nil
			}
			_, err := w.Write([]byte(v[0]))
			if err != nil {
				return err
			}
			v = v[1:]
			for _, line := range v {
				_, err = w.Write([]byte("\n"))
				if err != nil {
					return err
				}
				_, err := w.Write([]byte(line))
				if err != nil {
					return err
				}
			}
		default:
			return errors.New("invalid value to CSV producer")
		}
		return nil
	})

	v2API.TextJSONProducer = openapi_runtime.ProducerFunc(func(w io.Writer, data interface{}) error {
		var err error
		switch v := data.(type) {
		case []byte:
			_, err = w.Write(v)
		default:
			b, jerr := json.Marshal(data)
			if jerr != nil {
				return err
			}
			_, err = w.Write(b)
		}
		return err
	})

	// For local mode - we allow anything, otherwise we use the value specified in the config (e.g. AWS SSM)
	var apiHandler http.Handler
	if localMode {
		apiHandler = setupCORSHandlerLocal(
			wrapHandlers(
				// v1 API => /v3, python side is /v1 and /v2
				api.Serve(middlewareSetupfunc), swaggerSpec.BasePath(),
				// v2 API => /v4
				v2API.Serve(middlewareSetupfunc), v2SwaggerSpec.BasePath()))
	} else {
		apiHandler = setupCORSHandler(
			wrapHandlers(
				// v1 API => /v3, python side is /v1 and /v2
				api.Serve(middlewareSetupfunc), swaggerSpec.BasePath(),
				// v2 API => /v4
				v2API.Serve(middlewareSetupfunc), v2SwaggerSpec.BasePath()),
			configFile.AllowedOrigins)
	}
	return apiHandler
}

// setupCORSHandler sets up the CORS logic and creates the middleware HTTP handler
func setupCORSHandler(handler http.Handler, allowedOrigins []string) http.Handler {
	f := logrus.Fields{
		"functionName":   "cmd.setupCORSHandler",
		"allowedOrigins": strings.Join(allowedOrigins, ","),
	}

	log.WithFields(f).Debug("configuring allowed origins")
	c := cors.New(cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			u, err := url.Parse(origin)
			if err != nil {
				log.WithFields(f).WithError(err).Warn("cors parse origin issue")
				return false
			}

			// Ensure the origin is in our allowed list
			allowedOrigin := utils.HostInSlice(u.Hostname(), allowedOrigins)
			if allowedOrigin {
				// localhost with HTTP is allowed
				if strings.HasPrefix(u.Hostname(), "localhost") && u.Scheme == "http" {
					log.WithFields(f).Debugf("origin %s with protocol %s is allowed", u.Hostname(), u.Scheme)
					return true
				}

				// non-localhost with HTTPS is allowed
				if !strings.HasPrefix(u.Hostname(), "localhost") && u.Scheme == "https" {
					log.WithFields(f).Debugf("origin %s with protocol %s is allowed", u.Hostname(), u.Scheme)
					return true
				}

				log.WithFields(f).Debugf("origin %s with protocol %s is NOT allowed", u.Hostname(), u.Scheme)
				return false
			}

			log.WithFields(f).Warnf("origin %s is NOT allowed - not in allowed list: %v", u.Hostname(), allowedOrigins)
			return false
		},
		// Enable Debugging for testing, consider disabling in production
		Debug: false,
	})

	return c.Handler(handler)
}

// wrapHandlers routes the request to the appropriate handler
func wrapHandlers(v1 http.Handler, v1BasePath string, v2 http.Handler, v2BasePath string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		//log.Debugf("Path is: %s", r.URL.Path)
		if strings.HasPrefix(r.URL.Path, v1BasePath) {
			//log.Debugf("Routing to /v3 API handler...")
			v1.ServeHTTP(w, r)
		}
		if strings.HasPrefix(r.URL.Path, v2BasePath) {
			//log.Debugf("Routing to /v2 API handler...")
			v2.ServeHTTP(w, r)
		}
	})
}

// setupCORSHandlerLocal allows all origins and sets up the handler
func setupCORSHandlerLocal(handler http.Handler) http.Handler {
	f := logrus.Fields{
		"functionName": "cmd.setupCORSHandlerLocal",
	}

	log.WithFields(f).Debug("Allowing all origins")
	c := cors.New(cors.Options{
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE"},
		AllowedHeaders:   []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
		AllowOriginFunc:  func(origin string) bool { return true },
		//AllowOriginFunc:  func(origin string) bool { return true },
		// Enable Debugging for testing, consider disabling in production
		Debug: false,
	})

	return c.Handler(handler)
}

// LoggingResponseWriter is a wrapper around an http.ResponseWriter which captures the
// status code written to the response, so that it can be logged.
type LoggingResponseWriter struct {
	wrapped    http.ResponseWriter
	StatusCode int
	// Response content could also be captured here, but I was only interested in logging the response status code
}

// NewLoggingResponseWriter creates a new logging response writer
func NewLoggingResponseWriter(wrapped http.ResponseWriter) *LoggingResponseWriter {
	return &LoggingResponseWriter{wrapped: wrapped}
}

// Header returns the header
func (lrw *LoggingResponseWriter) Header() http.Header {
	return lrw.wrapped.Header()
}

// Write writes the contents
func (lrw *LoggingResponseWriter) Write(content []byte) (int, error) {
	return lrw.wrapped.Write(content)
}

// WriteHeader writes the header
func (lrw *LoggingResponseWriter) WriteHeader(statusCode int) {
	lrw.StatusCode = statusCode
	lrw.wrapped.WriteHeader(statusCode)
}

// setRequestIDHandler adds the x-request-id header, if missing
func setRequestIDHandler(next http.Handler) http.Handler {
	f := logrus.Fields{
		"functionName": "cmd.setRequestIDHandler",
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Set the x-request-id header value if it doesn't exist...
		if r.Header.Get(utils.XREQUESTID) == "" {
			requestID, err := uuid.NewV4()
			if err != nil {
				log.WithFields(f).WithError(err).Warn("unable to generate a UUID for x-request-id header")
			} else {
				r.Header.Set(utils.XREQUESTID, requestID.String())
			}
		}
		next.ServeHTTP(w, r)
	})
}

// responseLoggingMiddleware logs the responses from API endpoints
func responseLoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestStart(r.Header.Get(utils.XREQUESTID), r.Method)
		log.Debugf("BEGIN - %s %s", r.Method, r.URL.String())
		next.ServeHTTP(NewLoggingResponseWriter(w), r)
		if r.Response != nil {
			reqMetrics := getRequestMetrics(r.Header.Get(utils.XREQUESTID))
			if reqMetrics != nil {
				log.Debugf("END - %s %s - response code: %d response status: %s, elapsed: %v",
					r.Method, r.URL.String(), r.Response.StatusCode, r.Response.Status, reqMetrics.elapsed)
			} else {
				log.Debugf("END - %s %s - response code: %d response status: %s",
					r.Method, r.URL.String(), r.Response.StatusCode, r.Response.Status)
			}
			clearRequestMetrics(r.Header.Get(utils.XREQUESTID))
		} else {
			reqMetrics := getRequestMetrics(r.Header.Get(utils.XREQUESTID))
			if reqMetrics != nil {
				log.Debugf("END - %s %s, elapsed: %v", r.Method, r.URL.String(), reqMetrics.elapsed)
			} else {
				log.Debugf("END - %s %s", r.Method, r.URL.String())
			}
			clearRequestMetrics(r.Header.Get(utils.XREQUESTID))
		}
	})
}

// create user form http authorization token
// this function creates user if user does not exist and token is valid
func createUserFromRequest(authorizer auth.Authorizer, usersService users.Service, eventsService events.Service, r *http.Request) *http.Request {
	f := logrus.Fields{
		"functionName": "cmd.createUserFromRequest",
	}

	bToken := r.Header.Get("Authorization")
	if bToken == "" {
		return r
	}
	t := strings.Split(bToken, " ")
	if len(t) != 2 {
		log.WithFields(f).Warn("parsing of authorization header failed - expected two values separated by a space")
		return r
	}

	// parse user from the auth token
	claUser, err := authorizer.SecurityAuth(t[1], []string{})
	if err != nil {
		log.WithFields(f).WithError(err).Warn("parsing failed")
		return r
	}
	f["claUserName"] = claUser.Name
	f["claUserID"] = claUser.UserID
	f["claUserLFUsername"] = claUser.LFUsername
	f["claUserLFEmail"] = claUser.LFEmail
	f["claUserEmails"] = strings.Join(claUser.Emails, ",")

	// only needed if API called is /v4/user-from-token
	needToStoreUser := r.URL.Path == "/v4/user-from-token"

	// search if user exist in database by username
	userModel, err := usersService.GetUserByLFUserName(claUser.LFUsername)
	if err != nil {
		if _, ok := err.(*utils.UserNotFound); ok {
			log.WithFields(f).Debug("unable to locate user by lf-email")
		} else {
			log.WithFields(f).WithError(err).Warn("searching user by lf-username failed")
			return r
		}
	}
	// If found - just return
	if userModel != nil {
		if !needToStoreUser {
			return r
		}
		ctx := context.WithValue(r.Context(), "user", userModel) // nolint
		return r.WithContext(ctx)
	}

	// search if user exist in database by username
	userModel, err = usersService.GetUserByEmail(claUser.LFEmail)
	if err != nil {
		if _, ok := err.(*utils.UserNotFound); ok {
			log.WithFields(f).Debug("unable to locate user by lf-email")
		} else {
			log.WithFields(f).WithError(err).Warn("searching user by lf-email failed")
			return r
		}
	}
	// If found - just return
	if userModel != nil {
		if !needToStoreUser {
			return r
		}
		ctx := context.WithValue(r.Context(), "user", userModel) // nolint
		return r.WithContext(ctx)
	}

	// Attempt to create the user
	newUser := &models.User{
		LfEmail:    strfmt.Email(claUser.LFEmail),
		LfUsername: claUser.LFUsername,
		Username:   claUser.Name,
	}
	log.WithFields(f).Debug("creating new user")
	userModel, err = usersService.CreateUser(newUser, nil)
	if err != nil {
		log.WithFields(f).WithField("user", newUser).WithError(err).Warn("creating new user failed")
		return r
	}

	// Log the event
	eventsService.LogEvent(&events.LogEventArgs{
		EventType: events.UserCreated,
		UserID:    userModel.UserID,
		UserModel: userModel,
		EventData: &events.UserCreatedEventData{},
	})
	if !needToStoreUser {
		return r
	}
	ctx := context.WithValue(r.Context(), "user", userModel) // nolint
	return r.WithContext(ctx)
}
