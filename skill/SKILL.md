# LLM Captcha Solver - MCP Tool Guide

This document provides detailed instructions for LLMs on how to use the captcha solving tools via MCP protocol.

## Available Tools

You have access to three captcha solving tools:

1. `solve_recaptcha_v2` - Solve reCAPTCHA v2 challenges
2. `solve_recaptcha_v3` - Solve reCAPTCHA v3 challenges
3. `solve_geetest` - Solve GeeTest v3/v4 challenges

## General Workflow

1. **Identify the captcha type** on the target website
2. **Extract required parameters** from the page source or network requests
3. **Call the appropriate tool** with the extracted parameters
4. **Wait for the solution** (typically 30-120 seconds)
5. **Submit the solution** to the target website

---

## Tool 1: solve_recaptcha_v2

### Description
Solves reCAPTCHA v2 challenges (checkbox or invisible). Returns a `g-recaptcha-response` token.

### Required Parameters

- `websiteURL` (string, required): The full URL where the captcha is loaded
- `websiteKey` (string, required): The reCAPTCHA sitekey

### Optional Parameters

- `recaptchaDataSValue` (string): Value of `data-s` parameter (required for some Google services)
- `isInvisible` (boolean): Set to `true` for invisible reCAPTCHA (no checkbox)
- `userAgent` (string): Custom User-Agent string
- `cookies` (string): Cookies in format `key1=val1; key2=val2`
- `apiDomain` (string): Domain to load captcha from (`google.com` or `recaptcha.net`)

### How to Extract Parameters

#### 1. Finding websiteKey

**Method A: Search HTML source**
```html
<div class="g-recaptcha" data-sitekey="6Le-wvkSAAAAAPBMRTvw0Q4Muexq9bi0DJwx_mJ-"></div>
```
Extract the `data-sitekey` value.

**Method B: Search JavaScript**
```javascript
grecaptcha.render('container', {
  'sitekey': '6Le-wvkSAAAAAPBMRTvw0Q4Muexq9bi0DJwx_mJ-'
});
```

**Method C: Network requests**
Look for requests to `www.google.com/recaptcha/api2/anchor` or `api.js`, the sitekey is in the URL parameter `k=`.

#### 2. Finding recaptchaDataSValue (if needed)

Search for `data-s` attribute in the HTML:
```html
<script src="https://www.google.com/recaptcha/api.js" data-s="YOUR_DATA_S_VALUE"></script>
```

Or in the reCAPTCHA iframe URL:
```
https://www.google.com/recaptcha/api2/anchor?k=SITEKEY&co=...&hl=en&v=...&s=DATA_S_VALUE
```

#### 3. Detecting Invisible reCAPTCHA

Look for:
```html
<div class="g-recaptcha" data-sitekey="..." data-size="invisible"></div>
```
Or:
```javascript
grecaptcha.render('container', {
  'sitekey': '...',
  'size': 'invisible'
});
```

If found, set `isInvisible: true`.

### Example Usage

```javascript
// Basic reCAPTCHA v2
const result = await use_mcp_tool({
  server_name: "llm-captcha",
  tool_name: "solve_recaptcha_v2",
  arguments: {
    websiteURL: "https://example.com/login",
    websiteKey: "6Le-wvkSAAAAAPBMRTvw0Q4Muexq9bi0DJwx_mJ-"
  }
});

// Invisible reCAPTCHA with data-s
const result = await use_mcp_tool({
  server_name: "llm-captcha",
  tool_name: "solve_recaptcha_v2",
  arguments: {
    websiteURL: "https://accounts.google.com/signup",
    websiteKey: "6Lf8N8oZAAAAABbF...",
    recaptchaDataSValue: "s7dHQGfU...",
    isInvisible: true
  }
});
```

### Response Format

```json
{
  "gRecaptchaResponse": "03AGdBq27SvP8h7..."
}
```

### How to Submit the Solution

The `gRecaptchaResponse` token should be submitted in the form field named `g-recaptcha-response`:

```html
<input type="hidden" name="g-recaptcha-response" value="03AGdBq27SvP8h7...">
```

Or via JavaScript:
```javascript
document.getElementById('g-recaptcha-response').value = 'TOKEN';
```

---

## Tool 2: solve_recaptcha_v3

### Description
Solves reCAPTCHA v3 challenges (invisible, score-based). Returns a `g-recaptcha-response` token.

### Required Parameters

- `websiteURL` (string, required): The full URL where the captcha is loaded
- `websiteKey` (string, required): The reCAPTCHA sitekey
- `minScore` (number, required): Minimum required score: `0.3`, `0.7`, or `0.9`

### Optional Parameters

- `pageAction` (string): Action parameter (default: `verify`)
- `isEnterprise` (boolean): Set to `true` for Enterprise reCAPTCHA
- `apiDomain` (string): Domain to load captcha from (`google.com` or `recaptcha.net`)

### How to Extract Parameters

#### 1. Finding websiteKey

**Method A: Search for api.js script**
```html
<script src="https://www.google.com/recaptcha/api.js?render=6LfYZ2oUAAAAAH..."></script>
```
The sitekey is the value after `render=`.

**Method B: Search JavaScript for grecaptcha.execute**
```javascript
grecaptcha.execute('6LfYZ2oUAAAAAH...', {action: 'submit'})
```

#### 2. Finding pageAction

Look for the `action` parameter in `grecaptcha.execute()`:
```javascript
grecaptcha.execute('SITEKEY', {action: 'login'})
```

Common actions: `submit`, `login`, `verify`, `homepage`

#### 3. Detecting Enterprise reCAPTCHA

Look for `enterprise.js` instead of `api.js`:
```html
<script src="https://www.google.com/recaptcha/enterprise.js?render=SITEKEY"></script>
```

Or:
```javascript
grecaptcha.enterprise.execute('SITEKEY', {action: 'submit'})
```

If found, set `isEnterprise: true`.

#### 4. Determining minScore

This depends on the website's backend validation. Common values:
- `0.3` - Low security (most permissive)
- `0.7` - Medium security (recommended default)
- `0.9` - High security (strictest)

Start with `0.7` and adjust if the website rejects the token.

### Example Usage

```javascript
// Basic reCAPTCHA v3
const result = await use_mcp_tool({
  server_name: "llm-captcha",
  tool_name: "solve_recaptcha_v3",
  arguments: {
    websiteURL: "https://example.com/submit",
    websiteKey: "6LfYZ2oUAAAAAH...",
    minScore: 0.7,
    pageAction: "submit"
  }
});

// Enterprise reCAPTCHA v3
const result = await use_mcp_tool({
  server_name: "llm-captcha",
  tool_name: "solve_recaptcha_v3",
  arguments: {
    websiteURL: "https://enterprise-site.com",
    websiteKey: "6LfYZ2oUAAAAAH...",
    minScore: 0.9,
    pageAction: "login",
    isEnterprise: true
  }
});
```

### Response Format

```json
{
  "gRecaptchaResponse": "03AGdBq27SvP8h7..."
}
```

### How to Submit the Solution

Submit the token in the same way as reCAPTCHA v2, or pass it to the backend API that validates the token.

---

## Tool 3: solve_geetest

### Description
Solves GeeTest v3 or v4 challenges. Returns validation tokens.

### Required Parameters

- `websiteURL` (string, required): The full URL where the captcha is loaded
- `gt` (string, required): GeeTest gt value (or captcha_id for v4)

### Optional Parameters (v3)

- `challenge` (string, required for v3): GeeTest challenge value
- `geetestApiServerSubdomain` (string): Custom API domain (e.g., `api-na.geetest.com`)
- `userAgent` (string): Custom User-Agent
- `version` (number): Set to `3` for GeeTest v3

### Optional Parameters (v4)

- `version` (number, required): Set to `4` for GeeTest v4
- `initParameters` (object, required): Initialization parameters, must contain `captcha_id`

### How to Extract Parameters

#### 1. Detecting GeeTest Version

**GeeTest v3:**
```javascript
// Look for initGeetest function
initGeetest({
  gt: "c9c4facd1a6feeb80802222cbb74ca8e",
  challenge: "12ae8...",
  // ...
}, callback);
```

**GeeTest v4:**
```javascript
// Look for initGeetest4 function
initGeetest4({
  captchaId: "647f5ed2ed8acb4be36784e01556bb71",
  product: "bind"
}, callback);
```

Or check the script source:
- v3: `https://static.geetest.com/static/js/gt.0.5.0.js`
- v4: `https://static.geetest.com/v4/gt4.js`

#### 2. Finding gt / captcha_id

**Method A: Search JavaScript**
```javascript
var gt = "c9c4facd1a6feeb80802222cbb74ca8e";
```

**Method B: Network requests**
Look for requests to `api.geetest.com` or `gcaptcha4.geetest.com`, the gt/captcha_id is in the URL or response.

#### 3. Finding challenge (v3 ONLY)

⚠️ **CRITICAL: Challenge values are time-sensitive and single-use!**

**DO NOT extract challenge from page source or DOM** - it will be invalid!

**Correct Method:**

1. **Monitor network requests** when the page loads
2. **Find the challenge initialization request**, typically:
   - `GET https://api.geetest.com/get.php?gt=...`
   - `GET https://api.geetest.com/register.php?gt=...`
   - Or custom domain: `https://[custom-domain]/get.php?gt=...`

3. **Extract challenge from the response:**
   ```json
   {
     "success": 1,
     "challenge": "12ae8a57eb1e3b3590cf5d...",
     "gt": "c9c4facd1a6feeb80802222cbb74ca8e",
     "new_captcha": true
   }
   ```

4. **Immediately call the solve_geetest tool** with the fresh challenge value

**Important Rules:**
- ✅ Get a NEW challenge value for EACH solve request
- ✅ Call the tool immediately after getting the challenge
- ❌ NEVER reuse a challenge value
- ❌ NEVER extract challenge from HTML/DOM (it's already expired)

**Example workflow:**
```javascript
// Step 1: Get fresh challenge
const initResponse = await fetch('https://api.geetest.com/get.php?gt=c9c4facd...');
const initData = await initResponse.json();

// Step 2: Immediately solve with fresh challenge
const result = await use_mcp_tool({
  server_name: "llm-captcha",
  tool_name: "solve_geetest",
  arguments: {
    websiteURL: "https://example.com",
    gt: initData.gt,
    challenge: initData.challenge,  // Fresh challenge!
    version: 3
  }
});
```

#### 4. Finding geetestApiServerSubdomain (v3)

Look for the `api_server` parameter:
```javascript
initGeetest({
  gt: "...",
  challenge: "...",
  api_server: "api-na.geetest.com"  // Custom domain
}, callback);
```

If not specified, default is `api.geetest.com`.

#### 5. Finding initParameters (v4)

Extract all parameters passed to `initGeetest4()`:
```javascript
initGeetest4({
  captchaId: "647f5ed2ed8acb4be36784e01556bb71",
  product: "bind",
  language: "en",
  protocol: "https://"
}, callback);
```

### Example Usage

**GeeTest v3:**
```javascript
// First, get fresh challenge
const initResp = await fetch('https://api.geetest.com/get.php?gt=c9c4facd1a6feeb80802222cbb74ca8e');
const initData = await initResp.json();

// Immediately solve
const result = await use_mcp_tool({
  server_name: "llm-captcha",
  tool_name: "solve_geetest",
  arguments: {
    websiteURL: "https://example.com/login",
    gt: "c9c4facd1a6feeb80802222cbb74ca8e",
    challenge: initData.challenge,  // Fresh!
    version: 3
  }
});
```

**GeeTest v3 with custom API domain:**
```javascript
const result = await use_mcp_tool({
  server_name: "llm-captcha",
  tool_name: "solve_geetest",
  arguments: {
    websiteURL: "https://example.com",
    gt: "c9c4facd1a6feeb80802222cbb74ca8e",
    challenge: "12ae8a57eb1e3b3590cf5d...",
    geetestApiServerSubdomain: "api-na.geetest.com",
    version: 3
  }
});
```

**GeeTest v4:**
```javascript
const result = await use_mcp_tool({
  server_name: "llm-captcha",
  tool_name: "solve_geetest",
  arguments: {
    websiteURL: "https://example.com",
    gt: "647f5ed2ed8acb4be36784e01556bb71",
    version: 4,
    initParameters: {
      captcha_id: "647f5ed2ed8acb4be36784e01556bb71",
      product: "bind"
    }
  }
});
```

### Response Format

**GeeTest v3:**
```json
{
  "challenge": "12ae8a57eb1e3b3590cf5d...",
  "validate": "6f7a5c4b3d2e1a9f8e7d6c5b",
  "seccode": "6f7a5c4b3d2e1a9f8e7d6c5b|jordan"
}
```

**GeeTest v4:**
```json
{
  "captcha_id": "647f5ed2ed8acb4be36784e01556bb71",
  "captcha_output": "UrH-dHH3...",
  "gen_time": "1234567890",
  "lot_number": "abc123...",
  "pass_token": "xyz789..."
}
```

### How to Submit the Solution

**GeeTest v3:**
Submit the three values as form fields or JSON:
```javascript
{
  "geetest_challenge": result.challenge,
  "geetest_validate": result.validate,
  "geetest_seccode": result.seccode
}
```

**GeeTest v4:**
Submit all fields from the response to the backend validation endpoint.

---

## Common Troubleshooting

### Issue: "Task timeout" or "No solver available"

**Cause:** No Electron client is connected to the server.

**Solution:**
1. Start the Electron client: `cd client && npm start`
2. Connect to WebSocket: `ws://localhost:8080/ws`

### Issue: reCAPTCHA returns "Invalid site key"

**Cause:** Wrong `websiteKey` or `websiteURL`.

**Solution:**
- Verify the sitekey matches the one in the HTML
- Ensure `websiteURL` is the exact URL where the captcha appears (including protocol and path)

### Issue: GeeTest returns "Challenge expired"

**Cause:** Challenge value was reused or extracted from DOM.

**Solution:**
- Always get a fresh challenge from the initialization API
- Call the solve tool immediately after getting the challenge
- Never reuse challenge values

### Issue: reCAPTCHA v3 token rejected by website

**Cause:** Score too low for the website's requirements.

**Solution:**
- Increase `minScore` to `0.9`
- Check if the website uses Enterprise reCAPTCHA (set `isEnterprise: true`)

---

## Best Practices

1. **Always use the exact websiteURL** where the captcha appears
2. **For GeeTest v3, always fetch fresh challenge values** - never reuse or extract from DOM
3. **Start with default parameters** and only add optional ones if needed
4. **Wait for the full response** - solving typically takes 30-120 seconds
5. **Handle errors gracefully** - retry with fresh parameters if the first attempt fails
6. **For reCAPTCHA v3, start with minScore 0.7** and adjust based on website requirements

---

## API Endpoint Reference (for direct REST usage)

If MCP tools are unavailable, you can use the REST API directly:

**Create Task:**
```bash
POST http://localhost:8080/api/task
Content-Type: application/json

{
  "type": "RecaptchaV2Task",
  "websiteURL": "https://example.com",
  "websiteKey": "6Le-wvkSAAAAA..."
}
```

**Get Result:**
```bash
GET http://localhost:8080/api/task/{taskId}
```

Poll every 2 seconds until `status` is `completed` or `failed`.

---

## Summary

- Use `solve_recaptcha_v2` for checkbox and invisible reCAPTCHA v2
- Use `solve_recaptcha_v3` for score-based reCAPTCHA v3
- Use `solve_geetest` for GeeTest v3/v4
- **Always get fresh challenge values for GeeTest v3**
- Extract parameters from HTML source, JavaScript, or network requests
- Wait 30-120 seconds for solutions
- Submit solutions using the appropriate form fields or API endpoints
