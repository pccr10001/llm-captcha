package server

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
)

var supportedTypes = map[string]bool{
	"RecaptchaV2Task": true,
	"RecaptchaV3Task": true,
	"GeeTestTask":     true,
}

type CreateTaskRequest struct {
	Type string `json:"type" binding:"required"`

	// RecaptchaV2
	WebsiteURL          string `json:"websiteURL,omitempty"`
	WebsiteKey          string `json:"websiteKey,omitempty"`
	RecaptchaDataSValue string `json:"recaptchaDataSValue,omitempty"`
	IsInvisible         bool   `json:"isInvisible,omitempty"`
	UserAgent           string `json:"userAgent,omitempty"`
	Cookies             string `json:"cookies,omitempty"`
	ApiDomain           string `json:"apiDomain,omitempty"`

	// RecaptchaV3 additional
	MinScore     float64 `json:"minScore,omitempty"`
	PageAction   string  `json:"pageAction,omitempty"`
	IsEnterprise bool    `json:"isEnterprise,omitempty"`

	// GeeTest
	GT                        string          `json:"gt,omitempty"`
	Challenge                 string          `json:"challenge,omitempty"`
	GeetestApiServerSubdomain string          `json:"geetestApiServerSubdomain,omitempty"`
	Version                   int             `json:"version,omitempty"`
	InitParameters            json.RawMessage `json:"initParameters,omitempty"`
}

type TaskResponse struct {
	TaskID   string          `json:"taskId"`
	Status   string          `json:"status"`
	Solution json.RawMessage `json:"solution,omitempty"`
	Error    string          `json:"error,omitempty"`
}

func HandleCreateTask(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req CreateTaskRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if !supportedTypes[req.Type] {
			c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported task type: " + req.Type})
			return
		}

		// Validate required fields per type
		switch req.Type {
		case "RecaptchaV2Task":
			if req.WebsiteURL == "" || req.WebsiteKey == "" {
				c.JSON(http.StatusBadRequest, gin.H{"error": "websiteURL and websiteKey are required for RecaptchaV2Task"})
				return
			}
		case "RecaptchaV3Task":
			if req.WebsiteURL == "" || req.WebsiteKey == "" || req.MinScore == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "websiteURL, websiteKey, and minScore are required for RecaptchaV3Task"})
				return
			}
		case "GeeTestTask":
			if req.WebsiteURL == "" || req.GT == "" || req.Challenge == "" {
				if req.Version == 4 {
					if req.WebsiteURL == "" || req.GT == "" {
						c.JSON(http.StatusBadRequest, gin.H{"error": "websiteURL and gt are required for GeeTestTask v4"})
						return
					}
				} else {
					c.JSON(http.StatusBadRequest, gin.H{"error": "websiteURL, gt, and challenge are required for GeeTestTask"})
					return
				}
			}
		}

		// Marshal the full request as params (minus the type field)
		params, _ := json.Marshal(req)
		task := s.tasks.Create(req.Type, params)

		// Try to assign to an idle client immediately
		TryAssignTask(s.clients, task)

		c.JSON(http.StatusOK, TaskResponse{
			TaskID: task.ID,
			Status: task.Status,
		})
	}
}

func HandleGetTask(s *Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")
		task := s.tasks.Get(id)
		if task == nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "task not found"})
			return
		}

		resp := TaskResponse{
			TaskID:   task.ID,
			Status:   task.Status,
			Solution: task.Solution,
		}

		if task.Status == "failed" && task.Error != "" {
			resp.Error = task.Error
		}

		c.JSON(http.StatusOK, resp)
	}
}
