# Service Account Management - User Guide

## Table of Contents
1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Creating Service Accounts](#creating-service-accounts)
4. [Managing Permissions](#managing-permissions)
5. [IP Whitelist Configuration](#ip-whitelist-configuration)
6. [Secret Management](#secret-management)
7. [Client Implementation](#client-implementation)
8. [Troubleshooting](#troubleshooting)
9. [Security Best Practices](#security-best-practices)

---

## Overview

Service Accounts provide secure HMAC-based authentication for applications accessing GoRTMS APIs. Unlike user accounts that use JWT tokens, service accounts use cryptographic signatures to authenticate each API request.

### Why Use Service Accounts?
- **No token expiration** - No need to refresh tokens
- **Enhanced security** - Each request is individually signed
- **Fine-grained permissions** - Control exact API access per service
- **IP restrictions** - Limit access to specific networks
- **Audit trail** - Track service usage and last access times

### How HMAC Authentication Works
Each API request includes:
- `X-Service-ID`: Your service account identifier
- `X-Timestamp`: Current ISO 8601 timestamp
- `X-Signature`: HMAC-SHA256 signature of the request

---

## Quick Start

### 1. Create Your First Service Account

1. **Log in** to GoRTMS web interface
2. **Navigate** to "Service Accounts" in the sidebar
3. **Click** "Create Service" button
4. **Fill the form**:
   - **Name**: `my-app-production`
   - **Permissions**: Select `publish:orders` (example)
   - **IP Whitelist**: Leave empty for now
5. **Click** "Create Service"
6. **⚠️ IMPORTANT**: Copy the secret immediately - it will never be shown again!

### 2. Test Your Service Account

```bash
# Example API call using curl
curl -X POST "https://your-gortms.com/api/domains/orders/queues/pending/messages" \
  -H "Content-Type: application/json" \
  -H "X-Service-ID: my-app-production-240622-143022" \
  -H "X-Timestamp: 2024-06-22T14:30:22Z" \
  -H "X-Signature: sha256=abc123..." \
  -d '{"customer": "john", "amount": 100}'
```

---

## Creating Service Accounts

### Step-by-Step Creation

1. **Access the Interface**
   - Go to **Service Accounts** in the main navigation
   - Click **Create Service** button

2. **Service Configuration**
   ```
   Service Name*: my-payment-processor
   ```
   - Use descriptive names: `app-environment` format
   - Examples: `web-frontend-prod`, `data-sync-staging`

3. **Set Permissions**
   - Choose **Action**: `publish`, `consume`, `manage`, or `*` (all)
   - Choose **Domain**: specific domain name or `*` (all domains)
   - Click **Add** to include the permission

4. **Configure IP Whitelist (Optional)**
   - Add IP addresses that can use this service
   - Support for wildcards: `192.168.1.*`, `10.0.*`
   - Use `*` to allow all IPs (not recommended for production)

5. **Create and Save Secret**
   - Click **Create Service**
   - **Copy the secret immediately** - this is your only chance!
   - Store it securely (environment variables, key management system)

### Service Account Information

After creation, you'll see:
- **Service ID**: Unique identifier (e.g., `my-app-prod-240622-143022`)
- **Status**: Active, Disabled, or Not Used
- **Permissions**: List of granted permissions
- **Last Used**: When the service was last accessed
- **Creation Date**: When the account was created

---

## Managing Permissions

### Permission Format

Permissions follow the pattern: `action:domain`

### Available Actions

| Action | Description | API Access |
|--------|-------------|------------|
| `publish` | Send messages to queues | POST `/domains/{domain}/queues/{queue}/messages` |
| `consume` | Read messages from queues | GET `/domains/{domain}/queues/{queue}/messages` |
| `manage` | Manage consumer groups | Consumer group operations |
| `*` | All actions | Full API access |

### Domain Targeting

| Domain Value | Description | Example |
|--------------|-------------|---------|
| `orders` | Specific domain only | Access only "orders" domain |
| `*` | All domains | Access any domain |

### Permission Examples

```
publish:orders     → Can publish to "orders" domain only
consume:*          → Can consume from any domain  
manage:analytics   → Can manage consumer groups in "analytics"
*                  → Full access to everything
```

### Common Permission Patterns

**Web Application (Frontend)**
```
publish:orders
publish:notifications
consume:user-events
```

**Data Processing Service**
```
consume:raw-data
publish:processed-data
manage:processing-jobs
```

**Admin Dashboard**
```
*    (full access)
```

**Analytics Service**
```
consume:*
```

### Editing Permissions

1. **Find your service** in the Service Accounts list
2. **Click the Edit button** (pencil icon)
3. **Modify permissions**:
   - Add new permissions using the builder
   - Remove existing permissions with the X button
   - Toggle "Service Enabled" checkbox
4. **Click Save** to apply changes

---

## IP Whitelist Configuration

### Purpose
IP whitelisting restricts service account access to specific IP addresses or networks, adding an extra security layer.

### IP Format Options

| Format | Description | Example |
|--------|-------------|---------|
| Exact IP | Single IP address | `192.168.1.100` |
| Wildcard | IP range with asterisk | `192.168.1.*`, `10.0.*` |
| All IPs | Allow from anywhere | `*` |

### Configuration Examples

**Production Application (Fixed Server)**
```
192.168.1.50
```

**Development Environment (Local Network)**
```
192.168.1.*
10.0.*
```

**Cloud Services (Multiple IPs)**
```
203.0.113.10
203.0.113.11
203.0.113.12
```

**Development/Testing (No Restrictions)**
```
*
```

### Managing IP Whitelist

1. **During creation** or **while editing** a service
2. **Enter IP address** in the IP input field
3. **Click "Add IP"** to include it
4. **Remove IPs** by clicking the X button next to each entry
5. **Leave empty** for no IP restrictions (allows all IPs)

---

## Secret Management

### Understanding Secrets

- **Format**: 64-character hexadecimal string
- **Usage**: Used to sign HMAC signatures for authentication
- **Security**: Never transmitted in API calls, only used for signing
- **Visibility**: Shown only once during creation/rotation

### Secret Rotation

**When to rotate:**
- Suspected compromise
- Regular security maintenance (quarterly/yearly)
- Team member changes
- Security policy requirements

**How to rotate:**

1. **Find your service** in the Service Accounts list
2. **Click the Rotate button** (refresh icon)  
3. **Confirm the action** (this invalidates the old secret)
4. **Copy the new secret immediately**
5. **Update your applications** with the new secret
6. **Test the new secret** before completing deployment

**⚠️ Important Notes:**
- Old secret becomes invalid immediately
- Update all applications using this service account
- Test thoroughly in staging before production rotation

### Secret Storage Best Practices

**✅ Recommended:**
- Environment variables
- Key management systems (AWS KMS, HashiCorp Vault)
- Encrypted configuration files
- Container secrets

**❌ Avoid:**
- Hard-coding in source code
- Plain text configuration files
- Shared documents
- Email or chat messages

---

## Client Implementation

### JavaScript/Node.js Example

```javascript
import crypto from 'crypto';

class GoRTMSClient {
  constructor(serviceId, secret, baseURL) {
    this.serviceId = serviceId;
    this.secret = secret;
    this.baseURL = baseURL;
  }

  generateSignature(method, path, body, timestamp) {
    const message = `${method}\n${path}\n${body}\n${timestamp}`;
    const signature = crypto
      .createHmac('sha256', this.secret)
      .update(message)
      .digest('hex');
    return `sha256=${signature}`;
  }

  async publishMessage(domain, queue, message) {
    const timestamp = new Date().toISOString();
    const path = `/api/domains/${domain}/queues/${queue}/messages`;
    const body = JSON.stringify(message);
    
    const signature = this.generateSignature('POST', path, body, timestamp);

    const response = await fetch(`${this.baseURL}${path}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
        'X-Service-ID': this.serviceId,
        'X-Timestamp': timestamp,
        'X-Signature': signature
      },
      body
    });

    if (!response.ok) {
      throw new Error(`API error: ${response.status} ${response.statusText}`);
    }

    return await response.json();
  }

  async consumeMessages(domain, queue, options = {}) {
    const timestamp = new Date().toISOString();
    const queryParams = new URLSearchParams(options).toString();
    const path = `/api/domains/${domain}/queues/${queue}/messages${queryParams ? `?${queryParams}` : ''}`;
    
    const signature = this.generateSignature('GET', path, '', timestamp);

    const response = await fetch(`${this.baseURL}${path}`, {
      method: 'GET',
      headers: {
        'X-Service-ID': this.serviceId,
        'X-Timestamp': timestamp,
        'X-Signature': signature
      }
    });

    if (!response.ok) {
      throw new Error(`API error: ${response.status} ${response.statusText}`);
    }

    return await response.json();
  }
}

// Usage
const client = new GoRTMSClient(
  'my-app-prod-240622-143022',
  'a1b2c3d4e5f6789012345678901234567890abcdef123456789012345678901234',
  'https://your-gortms.com'
);

// Publish a message
await client.publishMessage('orders', 'pending', {
  customer: 'john@example.com',
  amount: 99.99,
  items: ['item1', 'item2']
});

// Consume messages
const messages = await client.consumeMessages('orders', 'pending', {
  timeout: 30,
  max: 10
});
```

### Go Example

```go
package main

import (
    "bytes"
    "crypto/hmac"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "io"
    "net/http"
    "time"
)

type GoRTMSClient struct {
    ServiceID string
    Secret    string
    BaseURL   string
}

func (c *GoRTMSClient) generateSignature(method, path, body, timestamp string) string {
    message := fmt.Sprintf("%s\n%s\n%s\n%s", method, path, body, timestamp)
    h := hmac.New(sha256.New, []byte(c.Secret))
    h.Write([]byte(message))
    signature := hex.EncodeToString(h.Sum(nil))
    return fmt.Sprintf("sha256=%s", signature)
}

func (c *GoRTMSClient) PublishMessage(domain, queue string, message interface{}) error {
    timestamp := time.Now().UTC().Format(time.RFC3339)
    path := fmt.Sprintf("/api/domains/%s/queues/%s/messages", domain, queue)
    
    bodyBytes, err := json.Marshal(message)
    if err != nil {
        return err
    }
    
    signature := c.generateSignature("POST", path, string(bodyBytes), timestamp)
    
    req, err := http.NewRequest("POST", c.BaseURL+path, bytes.NewBuffer(bodyBytes))
    if err != nil {
        return err
    }
    
    req.Header.Set("Content-Type", "application/json")
    req.Header.Set("X-Service-ID", c.ServiceID)
    req.Header.Set("X-Timestamp", timestamp)
    req.Header.Set("X-Signature", signature)
    
    client := &http.Client{Timeout: 30 * time.Second}
    resp, err := client.Do(req)
    if err != nil {
        return err
    }
    defer resp.Body.Close()
    
    if resp.StatusCode != http.StatusOK {
        body, _ := io.ReadAll(resp.Body)
        return fmt.Errorf("API error: %d %s - %s", resp.StatusCode, resp.Status, string(body))
    }
    
    return nil
}

// Usage
func main() {
    client := &GoRTMSClient{
        ServiceID: "my-app-prod-240622-143022",
        Secret:    "a1b2c3d4e5f6789012345678901234567890abcdef123456789012345678901234",
        BaseURL:   "https://your-gortms.com",
    }
    
    message := map[string]interface{}{
        "customer": "john@example.com",
        "amount":   99.99,
        "items":    []string{"item1", "item2"},
    }
    
    err := client.PublishMessage("orders", "pending", message)
    if err != nil {
        panic(err)
    }
    
    fmt.Println("Message published successfully!")
}
```

### Python Example

```python
import hmac
import hashlib
import json
import requests
from datetime import datetime
from urllib.parse import urlencode

class GoRTMSClient:
    def __init__(self, service_id, secret, base_url):
        self.service_id = service_id
        self.secret = secret
        self.base_url = base_url
    
    def generate_signature(self, method, path, body, timestamp):
        message = f"{method}\n{path}\n{body}\n{timestamp}"
        signature = hmac.new(
            self.secret.encode('utf-8'),
            message.encode('utf-8'),
            hashlib.sha256
        ).hexdigest()
        return f"sha256={signature}"
    
    def publish_message(self, domain, queue, message):
        timestamp = datetime.utcnow().isoformat() + 'Z'
        path = f"/api/domains/{domain}/queues/{queue}/messages"
        body = json.dumps(message)
        
        signature = self.generate_signature('POST', path, body, timestamp)
        
        headers = {
            'Content-Type': 'application/json',
            'X-Service-ID': self.service_id,
            'X-Timestamp': timestamp,
            'X-Signature': signature
        }
        
        response = requests.post(f"{self.base_url}{path}", headers=headers, data=body)
        response.raise_for_status()
        return response.json()
    
    def consume_messages(self, domain, queue, **options):
        timestamp = datetime.utcnow().isoformat() + 'Z'
        query_string = urlencode(options) if options else ''
        path = f"/api/domains/{domain}/queues/{queue}/messages"
        if query_string:
            path += f"?{query_string}"
        
        signature = self.generate_signature('GET', path, '', timestamp)
        
        headers = {
            'X-Service-ID': self.service_id,
            'X-Timestamp': timestamp,
            'X-Signature': signature
        }
        
        response = requests.get(f"{self.base_url}{path}", headers=headers)
        response.raise_for_status()
        return response.json()

# Usage
client = GoRTMSClient(
    service_id='my-app-prod-240622-143022',
    secret='a1b2c3d4e5f6789012345678901234567890abcdef123456789012345678901234',
    base_url='https://your-gortms.com'
)

# Publish message
client.publish_message('orders', 'pending', {
    'customer': 'john@example.com',
    'amount': 99.99,
    'items': ['item1', 'item2']
})

# Consume messages
messages = client.consume_messages('orders', 'pending', timeout=30, max=10)
```

---

## Troubleshooting

### Common Error Messages

#### 401 Unauthorized - "missing HMAC headers"
**Cause**: Required headers are missing from your request.

**Solution**: Ensure all three headers are present:
```javascript
headers: {
  'X-Service-ID': 'your-service-id',
  'X-Timestamp': '2024-06-22T14:30:22Z',
  'X-Signature': 'sha256=abc123...'
}
```

#### 401 Unauthorized - "timestamp outside valid window"
**Cause**: Your timestamp is too old or in the future.

**Solution**: 
- Use current UTC time in ISO 8601 format
- Check server time synchronization
- Default window is 5 minutes

```javascript
// Correct timestamp format
const timestamp = new Date().toISOString();
```

#### 401 Unauthorized - "invalid service"
**Cause**: Service ID doesn't exist or is disabled.

**Solution**:
- Verify service ID in GoRTMS interface
- Check if service is enabled
- Ensure you're using the correct environment

#### 401 Unauthorized - "invalid signature"
**Cause**: HMAC signature doesn't match expected value.

**Solution**:
- Verify secret is correct
- Check signature generation algorithm
- Ensure canonical request format: `METHOD\nPATH\nBODY\nTIMESTAMP`

#### 403 Forbidden - "insufficient permissions"
**Cause**: Service doesn't have permission for the requested action.

**Solution**:
- Check service permissions in GoRTMS interface
- Add required permission (e.g., `publish:domain`)
- Verify domain name matches exactly

#### 403 Forbidden - "IP not whitelisted"
**Cause**: Request comes from non-whitelisted IP address.

**Solution**:
- Check your current IP address
- Add IP to service whitelist
- Use `*` for development (not recommended for production)

### Debug Signature Generation

**Verify your canonical request string:**

```javascript
// Example canonical request
const method = 'POST';
const path = '/api/domains/orders/queues/pending/messages';
const body = '{"customer":"john","amount":100}';
const timestamp = '2024-06-22T14:30:22Z';

const canonicalRequest = `${method}\n${path}\n${body}\n${timestamp}`;
console.log('Canonical request:', canonicalRequest);

// Should output:
// POST
// /api/domains/orders/queues/pending/messages
// {"customer":"john","amount":100}
// 2024-06-22T14:30:22Z
```

**Test signature generation:**

```javascript
const crypto = require('crypto');

const secret = 'your-secret-here';
const canonicalRequest = 'POST\n/api/domains/orders/queues/pending/messages\n{"customer":"john","amount":100}\n2024-06-22T14:30:22Z';

const signature = crypto
  .createHmac('sha256', secret)
  .update(canonicalRequest)
  .digest('hex');

console.log('Generated signature:', `sha256=${signature}`);
```

### Testing Tools

**Use curl for quick testing:**

```bash
# Test with curl (replace values)
curl -X POST "https://your-gortms.com/api/domains/orders/queues/pending/messages" \
  -H "Content-Type: application/json" \
  -H "X-Service-ID: your-service-id" \
  -H "X-Timestamp: $(date -u +%Y-%m-%dT%H:%M:%SZ)" \
  -H "X-Signature: sha256=your-calculated-signature" \
  -d '{"test":"message"}' \
  -v
```

**Check service status:**

1. Go to Service Accounts page
2. Find your service in the list
3. Check status badge (Active/Disabled/Not Used)
4. Verify "Last Used" updates after successful requests

---

## Security Best Practices

### Secret Management

**✅ Do:**
- Store secrets in environment variables or key management systems
- Use different service accounts for different environments
- Rotate secrets regularly (quarterly or yearly)
- Monitor service account usage

**❌ Don't:**
- Hard-code secrets in source code
- Share secrets via email or chat
- Use the same secret across multiple environments
- Log secrets or include them in error messages

### Permission Design

**Principle of Least Privilege:**
- Grant only the minimum permissions needed
- Use specific domains instead of `*` when possible
- Separate read and write permissions across services

**Good Permission Examples:**
```
# Web frontend - only publishes user actions
publish:user-events

# Analytics service - only consumes data
consume:raw-data
consume:user-events

# Payment processor - specific domain access
publish:payments
consume:payment-confirmations
```

**Bad Permission Examples:**
```
# Too broad - unnecessary risk
*

# Mixed environments
publish:*
consume:*
```

### Network Security

**IP Whitelisting:**
- Use specific IPs for production services
- Whitelist load balancer IPs, not individual server IPs
- Use VPN or private networks when possible
- Avoid `*` in production environments

### Monitoring and Auditing

**Track Service Usage:**
- Monitor "Last Used" timestamps
- Set up alerts for unused services
- Review permissions quarterly
- Disable or delete unused services

**Log Analysis:**
- Monitor failed authentication attempts
- Alert on permission denied errors
- Track usage patterns for anomaly detection

### Development vs Production

**Development Environment:**
```javascript
// More permissive for development
const devService = {
  permissions: ['*'],
  ipWhitelist: ['*']
};
```

**Production Environment:**
```javascript
// Restricted for production
const prodService = {
  permissions: ['publish:orders', 'consume:notifications'],
  ipWhitelist: ['203.0.113.10', '203.0.113.11']
};
```

### Incident Response

**If a Secret is Compromised:**

1. **Immediately rotate** the secret in GoRTMS
2. **Update applications** with the new secret
3. **Review logs** for unauthorized usage
4. **Check permissions** and reduce if necessary
5. **Add IP restrictions** if not already present
6. **Document the incident** for future reference

**Regular Security Reviews:**

- Review all service accounts monthly
- Disable unused accounts
- Audit permission grants
- Update IP whitelists as infrastructure changes
- Test secret rotation procedures

---

## Advanced Topics

### Multi-Environment Strategy

**Recommended Structure:**
```
my-app-development-{timestamp}
my-app-staging-{timestamp}
my-app-production-{timestamp}
```

**Environment-Specific Permissions:**
- **Development**: Broad permissions for testing
- **Staging**: Production-like restrictions
- **Production**: Minimal required permissions

### High Availability Considerations

**Multiple Service Accounts:**
- Use different service accounts for different application instances
- Implement graceful secret rotation without downtime
- Monitor service account health and failover capabilities

### Integration Patterns

**Microservices Architecture:**
- One service account per microservice
- Domain-specific permissions per service
- Centralized secret management

**Event-Driven Systems:**
- Publisher services: `publish:events`
- Consumer services: `consume:events`
- Processor services: `consume:input` + `publish:output`
