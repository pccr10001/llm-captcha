# Human Captcha Solver for LLM

> **LLMs still need us to resolve captchas**

A multi-captcha automated solving system with Go backend + Electron client architecture, enabling LLMs to request human assistance for captcha challenges.

## Features

- Supports **reCAPTCHA v2**, **reCAPTCHA v3**, **GeeTest v3/v4**
- **REST API** interface for easy integration
- **MCP (Model Context Protocol)** support for direct LLM tool integration
- **WebSocket** real-time task distribution with low latency
- **Electron client** with real browser environment
- Persistent session management to accumulate trust and improve success rates

## Quick Start

### 1. Start the Server

```bash
cd server
go run main.go serve --port 8080
```

Server will start at `http://localhost:8080`.

### 2. Start the Electron Client

```bash
cd client
npm install
npm start
```

Enter WebSocket address `ws://localhost:8080/ws` in the settings page and click connect.

### 3. Use MCP Tools (Recommended)

Configure in Claude Desktop or other MCP clients:

```json
{
  "mcpServers": {
    "llm-captcha": {
      "command": "C:\\path\\to\\llm-catpcha\\server\\main.exe",
      "args": ["mcp"]
    }
  }
}
```

LLMs will automatically get three tools:
- `solve_recaptcha_v2` - Solve reCAPTCHA v2
- `solve_recaptcha_v3` - Solve reCAPTCHA v3
- `solve_geetest` - Solve GeeTest v3/v4

## API Usage

### Basic Workflow

1. **Create Task** - POST `/api/task`
2. **Poll Result** - GET `/api/task/{taskId}` (poll every 2 seconds until completed)

### Example: Solving reCAPTCHA v2

#### 1. Create Task

```bash
curl -X POST http://localhost:8080/api/task \
  -H "Content-Type: application/json" \
  -d '{
    "type": "RecaptchaV2Task",
    "websiteURL": "https://example.com/login",
    "websiteKey": "6Le-wvkSAAAAAPBMRTvw0Q4Muexq9bi0DJwx_mJ-"
  }'
```

**Response:**
```json
{
  "taskId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "pending"
}
```

#### 2. Poll Result

```bash
curl http://localhost:8080/api/task/550e8400-e29b-41d4-a716-446655440000
```

**In Progress:**
```json
{
  "taskId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "assigned"
}
```

**Completed:**
```json
{
  "taskId": "550e8400-e29b-41d4-a716-446655440000",
  "status": "completed",
  "solution": {
    "gRecaptchaResponse": "03AGdBq27..."
  }
}
```

## Supported Captcha Types

### 1. RecaptchaV2Task

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | ✓ | Must be `"RecaptchaV2Task"` |
| `websiteURL` | string | ✓ | Full URL of the page where captcha is loaded |
| `websiteKey` | string | ✓ | reCAPTCHA sitekey (data-sitekey) |
| `recaptchaDataSValue` | string | | Value of data-s parameter (may be required for Google services) |
| `isInvisible` | boolean | | Set to true for invisible reCAPTCHA (no checkbox) |
| `userAgent` | string | | Custom User-Agent string |
| `cookies` | string | | Cookie string in format: `key1=val1; key2=val2` |
| `apiDomain` | string | | API domain: `google.com` or `recaptcha.net` |

**Response Format:**
```json
{
  "gRecaptchaResponse": "03AGdBq27..."
}
```

### 2. RecaptchaV3Task

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | ✓ | Must be `"RecaptchaV3Task"` |
| `websiteURL` | string | ✓ | Full URL of the page where captcha is loaded |
| `websiteKey` | string | ✓ | reCAPTCHA sitekey |
| `minScore` | number | ✓ | Minimum required score: `0.3`, `0.7`, or `0.9` |
| `pageAction` | string | | Action parameter (from data-action or grecaptcha.execute) |
| `isEnterprise` | boolean | | Set to true for Enterprise reCAPTCHA |
| `apiDomain` | string | | API domain: `google.com` or `recaptcha.net` |

**Response Format:**
```json
{
  "gRecaptchaResponse": "03AGdBq27..."
}
```

### 3. GeeTestTask (v3)

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | ✓ | Must be `"GeeTestTask"` |
| `websiteURL` | string | ✓ | Full URL of the page where captcha is loaded |
| `gt` | string | ✓ | GeeTest gt value |
| `challenge` | string | ✓ | GeeTest challenge value (**must be fresh for each request**) |
| `geetestApiServerSubdomain` | string | | Custom API domain (e.g., `api-na.geetest.com`) |
| `userAgent` | string | | Custom User-Agent string |
| `version` | number | | Must be `3` |

**Response Format:**
```json
{
  "challenge": "12ae8...",
  "validate": "6f7a...",
  "seccode": "6f7a...|jordan"
}
```

### 4. GeeTestTask (v4)

| Parameter | Type | Required | Description |
|-----------|------|----------|-------------|
| `type` | string | ✓ | Must be `"GeeTestTask"` |
| `websiteURL` | string | ✓ | Full URL of the page where captcha is loaded |
| `gt` | string | ✓ | GeeTest captcha_id |
| `version` | number | ✓ | Must be `4` |
| `initParameters` | object | ✓ | Initialization parameters, must contain `captcha_id` |
| `userAgent` | string | | Custom User-Agent string |

**initParameters Example:**
```json
{
  "captcha_id": "647f5ed2ed8acb4be36784e01556bb71",
  "product": "bind"
}
```

**Response Format:**
```json
{
  "captcha_id": "647f5ed2ed8acb4be36784e01556bb71",
  "captcha_output": "...",
  "gen_time": "1234567890",
  "lot_number": "...",
  "pass_token": "..."
}
```

## Important Notes

### GeeTest Challenge Value Handling

⚠️ **GeeTest v3 `challenge` values are time-sensitive and single-use:**

- Challenge values obtained from backend API **must be sent to this service immediately**
- Once a challenge is loaded on the frontend (page renders captcha), it becomes invalid
- **You must get a new challenge value for each request**

**Correct Workflow:**
1. Monitor network requests when the website loads
2. Identify the API endpoint that gets a new challenge (usually `/get.php` or `/register.php`)
3. Call that endpoint to get a fresh challenge before each solve request
4. Immediately send the fresh challenge to this service's API

**Incorrect Workflow:**
❌ Extracting challenge from loaded page DOM → Value is already expired

## Task Status

| Status | Description |
|--------|-------------|
| `pending` | Task created, waiting for assignment to solver client |
| `assigned` | Task assigned to client, solving in progress |
| `completed` | Task completed, solution field contains result |
| `failed` | Task failed, error field contains error message |

**Timeout Settings:**
- Pending tasks: 120 seconds timeout (no solver available)
- Assigned tasks: 120 seconds timeout (solving failed)

## Architecture

```
┌─────────────┐         ┌──────────────┐         ┌─────────────────┐
│   LLM/API   │ ──REST──▶│  Go Server   │ ──WS───▶│ Electron Client │
│   Client    │ ◀────────│  (Task Queue)│ ◀───────│  (Human Solver) │
└─────────────┘         └──────────────┘         └─────────────────┘
                              │
                              │ MCP Protocol
                              ▼
                        ┌──────────────┐
                        │ Claude/LLM   │
                        │   Desktop    │
                        └──────────────┘
```

- **Go Server**: Task management, REST API, WebSocket distribution, MCP service
- **Electron Client**: Real browser environment, protocol interception, multi-window solving
- **MCP Integration**: LLMs can directly call tools without manual API requests

## Development

### Build Server

```bash
cd server
go build -o llm-captcha.exe main.go
```

### Package Client

```bash
cd client
npm run build
```

## License

MIT License

## Contributing

Issues and Pull Requests are welcome!
