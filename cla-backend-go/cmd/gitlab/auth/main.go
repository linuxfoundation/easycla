// Copyright The Linux Foundation and each contributor to CommunityBridge.
// SPDX-License-Identifier: MIT

package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/go-resty/resty/v2"
	log "github.com/linuxfoundation/easycla/cla-backend-go/logging"
	"github.com/xanzy/go-gitlab"
)

const (
	clientRedirectURI = "http://localhost:8080/gitlab/oauth/callback"
	clientAppID       = "18718b478096e6a257eda51414d0d446ad28866c15187aa765f602fe906d0b17"
	clientAppSecret   = "8dd14ace0eb0e4674b849b6fed4ce51bbcc456fc62d9149aff15353c1dda6327"
)

const (
	hookURL = "https://4c1ba3f4f3c1.ngrok.io/gitlab/events"
)

type OauthSuccessResponse struct {
	AccessToken  string `json:"access_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int    `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
	CreatedAt    int    `json:"created_at"`
}

var passingUsers = map[string]bool{
	"deniskurov@gmail.com": true,
}

func main() {
	r := gin.Default()
	r.GET("/gitlab/sign", func(c *gin.Context) {
		email := c.Query("email")
		if email == "" {
			c.JSON(400, gin.H{
				"message": "email is required parameter",
			})
			return
		}

		projectID := c.Query("project_id")
		if projectID == "" {
			c.JSON(400, gin.H{
				"message": "projectID is required parameter",
			})
			return
		}

		lastCommitSha := c.Query("sha")
		if lastCommitSha == "" {
			c.JSON(400, gin.H{
				"message": "sha is required parameter",
			})
			return
		}

		projectIDInt, err := strconv.Atoi(projectID)
		if err != nil {
			log.Error("project id conversion failed ", err)
			c.JSON(400, gin.H{
				"message": "project id conversion",
			})
			return
		}

		if err := setCommitStatus(projectIDInt, lastCommitSha, email, string(gitlab.Success)); err != nil {
			log.Error("setting commit status failed", err)
			c.JSON(500, gin.H{
				"message": "setting commit status failed",
			})
			return
		}

		log.Infof("email to sign is : %s", email)
		log.Infof("project id : %s, sha : %s", projectID, lastCommitSha)

		c.JSON(http.StatusOK, gin.H{
			"message": fmt.Sprintf("user : %s, signed for project : %s", email, projectID),
		})

	})

	r.POST("/gitlab/events", func(c *gin.Context) {
		jsonData, err := ioutil.ReadAll(c.Request.Body)
		event, err := gitlab.ParseWebhook(gitlab.EventTypeMergeRequest, jsonData)
		if err != nil {
			log.Error("parsing json body failed", err)
			c.JSON(400, gin.H{
				"message": "code is required parameter",
			})
			return
		}

		mergeEvent, ok := event.(*gitlab.MergeEvent)
		if !ok {
			c.JSON(400, gin.H{
				"message": "type cast failed",
			})
			return
		}

		if mergeEvent.ObjectAttributes.State != "opened" {
			c.JSON(200, gin.H{
				"message": "only interested in opened events",
			})
			return
		}

		projectName := mergeEvent.Project.Name
		projectID := mergeEvent.Project.ID

		mergeID := mergeEvent.ObjectAttributes.IID
		lastCommitSha := mergeEvent.ObjectAttributes.LastCommit.ID
		lastCommitMessage := mergeEvent.ObjectAttributes.LastCommit.Message

		authorName := mergeEvent.ObjectAttributes.LastCommit.Author.Name
		authorEmail := mergeEvent.ObjectAttributes.LastCommit.Author.Email

		log.Printf("Received MR (%d) for Project %s:%d", mergeID, projectName, projectID)
		log.Printf("last commit : %s : %s", lastCommitSha, lastCommitMessage)
		log.Printf("author name : %s, author email : %s", authorName, authorEmail)

		if err := setCommitStatus(projectID, lastCommitSha, authorEmail, ""); err != nil {
			log.Error("setting commit status failed", err)
			c.JSON(500, gin.H{
				"message": "setting commit status failed",
			})
			return
		}

		//empJSON, err := json.MarshalIndent(mergeEvent, "", "  ")
		//if err != nil {
		//	log.Fatalf(err.Error())
		//}
		//fmt.Printf("MarshalIndent funnction output %s\n", string(empJSON))
		c.JSON(http.StatusOK, gin.H{})

	})
	r.GET("/gitlab/oauth/callback", func(c *gin.Context) {
		code := c.Query("code")
		if code == "" {
			c.JSON(400, gin.H{
				"message": "code is required parameter",
			})
			return
		}

		state := c.Query("state")
		if state == "" {
			c.JSON(400, gin.H{
				"message": "state is required parameter",
			})
			return
		}
		log.Printf("received code : %s, STATE: %s", code, state)

		client := resty.New()
		params := map[string]string{
			"client_id":     clientAppID,
			"client_secret": clientAppSecret,
			"code":          code,
			"grant_type":    "authorization_code",
			"redirect_uri":  clientRedirectURI,
		}

		resp, err := client.R().
			SetQueryParams(params).
			SetResult(&OauthSuccessResponse{}).
			Post("https://gitlab.com/oauth/token")

		if err != nil {
			c.JSON(500, gin.H{
				"message": fmt.Sprintf("getting the token failed : %v", err),
			})
			return
		}

		result := resp.Result().(*OauthSuccessResponse)
		accessToken := result.AccessToken

		err = registerWebHooksForUserProjects(accessToken)
		if err != nil {
			log.Error("register webhook ", err)
		}

		respData := gin.H{
			"message": "OK",
			"data":    result,
		}

		if err != nil {
			respData["error"] = err
		}

		c.JSON(200, respData)
	})
	r.Run() // listen and serve on 0.0.0.0:8080 (for windows "localhost:8080")
}

func registerWebHooksForUserProjects(accessToken string) error {
	gitlabClient, err := gitlab.NewOAuthClient(accessToken)
	if err != nil {
		return fmt.Errorf("creating client failed : %v", err)
	}

	user, _, err := gitlabClient.Users.CurrentUser()
	if err != nil {
		return fmt.Errorf("fetching current user failed : %v", err)
	}

	log.Infof("fetched current user : %s", user.Name)

	projects, _, err := gitlabClient.Projects.ListUserProjects(user.ID, &gitlab.ListProjectsOptions{})
	if err != nil {
		return fmt.Errorf("listing projects failed : %v", err)
	}

	log.Printf("we fetched : %d projects for the account", len(projects))

	for _, p := range projects {
		log.Println("**********************")
		log.Println("Name : ", p.Name)
		log.Println("ID: ", p.ID)
		log.Infof("adding webhook to the project : %s (%d)", p.Name, p.ID)
		if err := addCLAHookToProject(gitlabClient, p.ID); err != nil {
			return fmt.Errorf("adding hook to the project : %s (%d) failed : %v", p.Name, p.ID, err)
		}
	}

	return nil
}

func addCLAHookToProject(gitlabClient *gitlab.Client, projectID int) error {
	_, _, err := gitlabClient.Projects.AddProjectHook(projectID, &gitlab.AddProjectHookOptions{
		URL:                   gitlab.String(hookURL),
		MergeRequestsEvents:   gitlab.Bool(true),
		EnableSSLVerification: gitlab.Bool(false),
	})
	return err
}

func setCommitStatus(projectID interface{}, commitSha string, userEmail string, forceState string) error {
	accessToken := os.Getenv("GITLAB_ACCESS_TOKEN")
	if accessToken == "" {
		return fmt.Errorf("GITLAB_ACCESS_TOKEN is required")
	}

	gitlabClient, err := gitlab.NewOAuthClient(accessToken)
	if err != nil {
		return fmt.Errorf("creating client failed : %v", err)
	}

	setState := gitlab.Failed

	if forceState == "" {
		if passingUsers[userEmail] {
			setState = gitlab.Success
		}
	} else {
		setState = gitlab.BuildStateValue(forceState)
	}

	options := &gitlab.SetCommitStatusOptions{
		State:       setState,
		Name:        gitlab.String("easyCLA Bot"),
		Description: gitlab.String(getDescription(setState)),
	}

	if setState == gitlab.Failed {
		options.TargetURL = gitlab.String(getTargetURL(projectID, commitSha, userEmail))
	}

	_, _, err = gitlabClient.Commits.SetCommitStatus(projectID, commitSha, options)
	if err != nil {
		return fmt.Errorf("setting commit status for the sha failed : %v", err)
	}

	return nil
}

func getDescription(status gitlab.BuildStateValue) string {
	if status == gitlab.Failed {
		return "User hasn't signed CLA"
	}
	return "User signed CLA"
}

func getTargetURL(projectID interface{}, lastCommitSha, email string) string {
	base := "http://localhost:8080/gitlab/sign"

	projectIDInt := projectID.(int)
	projectIDStr := strconv.Itoa(projectIDInt)

	params := url.Values{}
	params.Add("project_id", projectIDStr)
	params.Add("sha", lastCommitSha)
	params.Add("email", email)

	return base + "?" + params.Encode()
}
