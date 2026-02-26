package mcp

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

// JSON-RPC types
type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      interface{}     `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      interface{} `json:"id,omitempty"`
	Result  interface{} `json:"result,omitempty"`
	Error   *rpcError   `json:"error,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type serverInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type initializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	Capabilities    serverCapabilities `json:"capabilities"`
	ServerInfo      serverInfo         `json:"serverInfo"`
}

type serverCapabilities struct {
	Tools *toolsCapability `json:"tools,omitempty"`
}

type toolsCapability struct{}

type tool struct {
	Name        string      `json:"name"`
	Description string      `json:"description"`
	InputSchema inputSchema `json:"inputSchema"`
}

type inputSchema struct {
	Type       string                 `json:"type"`
	Properties map[string]property    `json:"properties"`
	Required   []string               `json:"required"`
}

type property struct {
	Type        string      `json:"type"`
	Description string      `json:"description"`
	Enum        []float64   `json:"enum,omitempty"`
	Default     interface{} `json:"default,omitempty"`
}

type toolsListResult struct {
	Tools []tool `json:"tools"`
}

type callToolParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type callToolResult struct {
	Content []textContent `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

// API types
type createTaskReq struct {
	Type string `json:"type"`
	// All other fields are passed through as-is
}

type createTaskResp struct {
	TaskID string `json:"taskId"`
	Status string `json:"status"`
}

type getTaskResp struct {
	TaskID   string          `json:"taskId"`
	Status   string          `json:"status"`
	Solution json.RawMessage `json:"solution,omitempty"`
	Error    string          `json:"error,omitempty"`
}

var mcpTools = []tool{
	{
		Name:        "solve_recaptcha_v2",
		Description: "Solve a reCAPTCHA v2 challenge. Dispatches the task to a human solver via Electron client and returns the g-recaptcha-response token.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]property{
				"websiteURL":          {Type: "string", Description: "The full URL of target web page where the captcha is loaded"},
				"websiteKey":          {Type: "string", Description: "reCAPTCHA sitekey (data-sitekey)"},
				"recaptchaDataSValue": {Type: "string", Description: "Value of data-s parameter. May be required for Google services"},
				"isInvisible":         {Type: "boolean", Description: "True for invisible reCAPTCHA (no checkbox, challenge appears directly)"},
				"userAgent":           {Type: "string", Description: "Browser User-Agent to use when loading the captcha"},
				"cookies":             {Type: "string", Description: "Cookies in format: key1=val1; key2=val2"},
				"apiDomain":           {Type: "string", Description: "Domain to load captcha from: google.com or recaptcha.net. Default: google.com"},
			},
			Required: []string{"websiteURL", "websiteKey"},
		},
	},
	{
		Name:        "solve_recaptcha_v3",
		Description: "Solve a reCAPTCHA v3 challenge. Returns a g-recaptcha-response token with the requested minimum score.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]property{
				"websiteURL":   {Type: "string", Description: "The full URL of target web page where the captcha is loaded"},
				"websiteKey":   {Type: "string", Description: "reCAPTCHA sitekey (data-sitekey)"},
				"minScore":     {Type: "number", Description: "Required score value: 0.3, 0.7, or 0.9", Enum: []float64{0.3, 0.7, 0.9}},
				"pageAction":   {Type: "string", Description: "Action parameter from data-action or grecaptcha.execute options"},
				"isEnterprise": {Type: "boolean", Description: "True for Enterprise reCAPTCHA (uses enterprise.js instead of api.js)"},
				"apiDomain":    {Type: "string", Description: "Domain to load captcha from: google.com or recaptcha.net. Default: google.com"},
			},
			Required: []string{"websiteURL", "websiteKey", "minScore"},
		},
	},
	{
		Name:        "solve_geetest",
		Description: "Solve a GeeTest captcha (v3 or v4). Returns the validation tokens.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]property{
				"websiteURL":                {Type: "string", Description: "The full URL of target web page where the captcha is loaded"},
				"gt":                        {Type: "string", Description: "GeeTest gt value"},
				"challenge":                 {Type: "string", Description: "GeeTest challenge value (required for v3)"},
				"geetestApiServerSubdomain": {Type: "string", Description: "Custom GeeTest API domain (v3 only), e.g. api-na.geetest.com"},
				"userAgent":                 {Type: "string", Description: "Browser User-Agent to use"},
				"version":                   {Type: "number", Description: "GeeTest version: 3 or 4. Default: 3"},
				"initParameters":            {Type: "object", Description: "Required for v4. Parameters passed to initGeetest4, must contain captcha_id"},
			},
			Required: []string{"websiteURL", "gt"},
		},
	},
}

var toolTypeMap = map[string]string{
	"solve_recaptcha_v2": "RecaptchaV2Task",
	"solve_recaptcha_v3": "RecaptchaV3Task",
	"solve_geetest":      "GeeTestTask",
}

func RunStdio(serverURL string) {
	reader := bufio.NewReader(os.Stdin)

	for {
		line, err := reader.ReadBytes('\n')
		if err != nil {
			if err == io.EOF {
				return
			}
			fmt.Fprintf(os.Stderr, "read error: %v\n", err)
			return
		}

		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}

		var req jsonRPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			writeResponse(jsonRPCResponse{
				JSONRPC: "2.0",
				Error:   &rpcError{Code: -32700, Message: "Parse error"},
			})
			continue
		}

		resp := handleRequest(req, serverURL)
		if resp != nil {
			writeResponse(*resp)
		}
	}
}

func handleRequest(req jsonRPCRequest, serverURL string) *jsonRPCResponse {
	switch req.Method {
	case "initialize":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result: initializeResult{
				ProtocolVersion: "2024-11-05",
				Capabilities: serverCapabilities{
					Tools: &toolsCapability{},
				},
				ServerInfo: serverInfo{
					Name:    "llm-captcha",
					Version: "1.0.0",
				},
			},
		}

	case "notifications/initialized":
		return nil

	case "tools/list":
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  toolsListResult{Tools: mcpTools},
		}

	case "tools/call":
		var params callToolParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Error:   &rpcError{Code: -32602, Message: "Invalid params"},
			}
		}

		taskType, ok := toolTypeMap[params.Name]
		if !ok {
			return &jsonRPCResponse{
				JSONRPC: "2.0",
				ID:      req.ID,
				Result: callToolResult{
					Content: []textContent{{Type: "text", Text: "Unknown tool: " + params.Name}},
					IsError: true,
				},
			}
		}

		result := solveCaptcha(serverURL, taskType, params.Arguments)
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Result:  result,
		}

	default:
		return &jsonRPCResponse{
			JSONRPC: "2.0",
			ID:      req.ID,
			Error:   &rpcError{Code: -32601, Message: "Method not found: " + req.Method},
		}
	}
}

func solveCaptcha(serverURL string, taskType string, args json.RawMessage) callToolResult {
	// Build the create-task request: merge "type" into the arguments object
	var argsMap map[string]interface{}
	if err := json.Unmarshal(args, &argsMap); err != nil {
		return callToolResult{
			Content: []textContent{{Type: "text", Text: "Invalid arguments: " + err.Error()}},
			IsError: true,
		}
	}
	argsMap["type"] = taskType
	body, _ := json.Marshal(argsMap)

	resp, err := http.Post(serverURL+"/api/task", "application/json", bytes.NewReader(body))
	if err != nil {
		return callToolResult{
			Content: []textContent{{Type: "text", Text: "Failed to create task: " + err.Error()}},
			IsError: true,
		}
	}
	defer resp.Body.Close()

	var createResp createTaskResp
	if err := json.NewDecoder(resp.Body).Decode(&createResp); err != nil {
		return callToolResult{
			Content: []textContent{{Type: "text", Text: "Failed to parse create response: " + err.Error()}},
			IsError: true,
		}
	}

	// Poll for result
	timeout := time.After(120 * time.Second)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return callToolResult{
				Content: []textContent{{Type: "text", Text: "Task timed out after 120 seconds"}},
				IsError: true,
			}
		case <-ticker.C:
			pollResp, err := http.Get(serverURL + "/api/task/" + createResp.TaskID)
			if err != nil {
				continue
			}

			var taskResp getTaskResp
			if err := json.NewDecoder(pollResp.Body).Decode(&taskResp); err != nil {
				pollResp.Body.Close()
				continue
			}
			pollResp.Body.Close()

			switch taskResp.Status {
			case "completed":
				if taskResp.Solution != nil {
					// Return the solution JSON as-is
					resultObj := map[string]interface{}{
						"taskId":   taskResp.TaskID,
						"solution": json.RawMessage(taskResp.Solution),
					}
					resultJSON, _ := json.Marshal(resultObj)
					return callToolResult{
						Content: []textContent{{Type: "text", Text: string(resultJSON)}},
					}
				}
				return callToolResult{
					Content: []textContent{{Type: "text", Text: "Task completed but no solution found"}},
					IsError: true,
				}
			case "failed":
				errMsg := taskResp.Error
				if errMsg == "" {
					errMsg = "unknown error"
				}
				return callToolResult{
					Content: []textContent{{Type: "text", Text: "Task failed: " + errMsg}},
					IsError: true,
				}
			}
		}
	}
}

func writeResponse(resp jsonRPCResponse) {
	data, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "marshal error: %v\n", err)
		return
	}
	fmt.Fprintf(os.Stdout, "%s\n", data)
}
