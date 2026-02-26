package server

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/google/uuid"
)

type Task struct {
	ID          string          `json:"taskId"`
	Type        string          `json:"type"`
	Status      string          `json:"status"`
	Params      json.RawMessage `json:"params"`
	Solution    json.RawMessage `json:"solution,omitempty"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	AssignedAt  *time.Time      `json:"assignedAt,omitempty"`
	CompletedAt *time.Time      `json:"completedAt,omitempty"`
	AssignedTo  string          `json:"-"`
	Done        chan struct{}    `json:"-"`
}

// --- Request param types ---

type RecaptchaV2Params struct {
	WebsiteURL          string `json:"websiteURL"`
	WebsiteKey          string `json:"websiteKey"`
	RecaptchaDataSValue string `json:"recaptchaDataSValue,omitempty"`
	IsInvisible         bool   `json:"isInvisible,omitempty"`
	UserAgent           string `json:"userAgent,omitempty"`
	Cookies             string `json:"cookies,omitempty"`
	ApiDomain           string `json:"apiDomain,omitempty"`
}

type RecaptchaV3Params struct {
	WebsiteURL   string  `json:"websiteURL"`
	WebsiteKey   string  `json:"websiteKey"`
	MinScore     float64 `json:"minScore"`
	PageAction   string  `json:"pageAction,omitempty"`
	IsEnterprise bool    `json:"isEnterprise,omitempty"`
	ApiDomain    string  `json:"apiDomain,omitempty"`
}

type GeeTestParams struct {
	WebsiteURL                string          `json:"websiteURL"`
	GT                        string          `json:"gt"`
	Challenge                 string          `json:"challenge"`
	GeetestApiServerSubdomain string          `json:"geetestApiServerSubdomain,omitempty"`
	UserAgent                 string          `json:"userAgent,omitempty"`
	Version                   int             `json:"version,omitempty"` // 3 or 4, default 3
	InitParameters            json.RawMessage `json:"initParameters,omitempty"`
}

// --- Solution types ---

type RecaptchaV2Solution struct {
	GRecaptchaResponse string `json:"gRecaptchaResponse"`
}

type RecaptchaV3Solution struct {
	GRecaptchaResponse string `json:"gRecaptchaResponse"`
}

type GeeTestV3Solution struct {
	Challenge string `json:"challenge"`
	Validate  string `json:"validate"`
	Seccode   string `json:"seccode"`
}

type GeeTestV4Solution struct {
	CaptchaID     string `json:"captcha_id"`
	CaptchaOutput string `json:"captcha_output"`
	GenTime       string `json:"gen_time"`
	LotNumber     string `json:"lot_number"`
	PassToken     string `json:"pass_token"`
}

type TaskStore struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	pending []string
}

func NewTaskStore() *TaskStore {
	return &TaskStore{
		tasks: make(map[string]*Task),
	}
}

func (s *TaskStore) Create(taskType string, params json.RawMessage) *Task {
	t := &Task{
		ID:        uuid.New().String(),
		Type:      taskType,
		Params:    params,
		Status:    "pending",
		CreatedAt: time.Now(),
		Done:      make(chan struct{}),
	}

	s.mu.Lock()
	s.tasks[t.ID] = t
	s.pending = append(s.pending, t.ID)
	s.mu.Unlock()

	return t
}

func (s *TaskStore) Get(id string) *Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tasks[id]
}

func (s *TaskStore) AssignPending(clientID string) *Task {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, id := range s.pending {
		t := s.tasks[id]
		if t != nil && t.Status == "pending" {
			now := time.Now()
			t.Status = "assigned"
			t.AssignedTo = clientID
			t.AssignedAt = &now
			s.pending = append(s.pending[:i], s.pending[i+1:]...)
			return t
		}
	}
	return nil
}

func (s *TaskStore) Complete(id string, solution json.RawMessage) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.tasks[id]
	if t == nil || (t.Status != "assigned" && t.Status != "pending") {
		return false
	}
	now := time.Now()
	t.Status = "completed"
	t.Solution = solution
	t.CompletedAt = &now
	close(t.Done)
	return true
}

func (s *TaskStore) Fail(id, errMsg string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	t := s.tasks[id]
	if t == nil || (t.Status != "assigned" && t.Status != "pending") {
		return false
	}
	now := time.Now()
	t.Status = "failed"
	t.Error = errMsg
	t.CompletedAt = &now
	close(t.Done)
	return true
}

func (s *TaskStore) TimeoutExpired(timeout time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, t := range s.tasks {
		if t.Status == "assigned" && t.AssignedAt != nil && now.Sub(*t.AssignedAt) > timeout {
			t.Status = "failed"
			t.Error = "timeout"
			t.CompletedAt = &now
			select {
			case <-t.Done:
			default:
				close(t.Done)
			}
		}
		if t.Status == "pending" && now.Sub(t.CreatedAt) > timeout {
			t.Status = "failed"
			t.Error = "no solver available"
			t.CompletedAt = &now
			select {
			case <-t.Done:
			default:
				close(t.Done)
			}
		}
	}
}
