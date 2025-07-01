# GoRTMS - Real-Time Messaging System

GoRTMS is a message broker written in Go that implements a dual-layer messaging architecture. The system separates real-time message flow observability from durable message processing through distinct consumption patterns and provides enterprise-grade reliability features with comprehensive system monitoring.

## Architecture Overview

### Dual-Layer Processing

**Processing Layer**: Durable consumer groups with position-based consumption and acknowledgment tracking. This layer handles persistent message processing with replay capabilities and guaranteed delivery semantics.

**Observability Layer**: Real-time WebSocket connections for message flow visibility. This layer provides a "see-through" window into message streams for debugging and development without affecting the core processing pipeline.

### Key Features

- **Flow Control Mechanism**: Command-data channel pattern where consumers explicitly request message quantities
- **Position-Based Replay**: Consumer groups maintain independent read positions for complete message history access
- **Circuit Breaker Pattern**: Automatic failure isolation with configurable thresholds
- **Exponential Backoff Retry**: Smart retry mechanism with configurable delays and limits
- **Dual Authentication**: JWT for user sessions and HMAC-SHA256 for service-to-service communication
- **Resource Monitoring**: Built-in system metrics and performance tracking

## Requirements

- **Go 1.24+** (recommended for swiss map optimizations)
- **Node.js 16+** and **npm 8+** (for web interface)

## Installation and Setup

### 1. Clone and Build

```bash
# Clone repository
git clone https://github.com/ajkula/GoRTMS.git
cd GoRTMS

# Install Go dependencies
go mod tidy

# Build web interface
cd web
npm install
npm run build
cd ..

# Compile application
go build -o gortms.exe cmd/server/main.go
```

### 2. Configuration

Generate default configuration:

```bash
.\gortms.exe --generate-config
```

This creates a `config.yaml` file. Key configuration sections:

```yaml
general:
  logLevel: "info"

http:
  enabled: true
  address: "0.0.0.0"
  port: 8080

security:
  enableAuthentication: false  # Set to true for production
  hmac:
    timestampWindow: "5m"
```

### 3. Start Server

```bash
.\gortms.exe --config=config.yaml
```

Expected output:
```
Starting GoRTMS...
Node ID: node1
Data directory: ./data
HTTP server listening on 0.0.0.0:8080
GoRTMS started successfully
```

### 4. Access Web Interface

Navigate to `http://localhost:8080/ui/` for the management interface.

## Core Concepts

### Domains and Queues

**Domain**: A logical namespace defining message schemas and containing related queues. Domains enforce message validation through configurable schemas.

**Queue**: A message stream within a domain with specific processing characteristics including persistence, retry policies, and circuit breaker configuration.

### Consumer Groups

Consumer groups provide scalable message consumption with the following characteristics:

- **Position Tracking**: Each group maintains an independent read position for replay capability
- **Multiple Consumers**: Groups support multiple active consumers sharing message load
- **TTL Management**: Unused groups expire automatically to prevent resource accumulation
- **Independent Processing**: Groups consume messages independently without affecting each other

## Authentication

### Development Setup

For development, disable authentication in `config.yaml`:

```yaml
security:
  enableAuthentication: false
```

### Production Setup

#### JWT Authentication

[JWT docs](docs/usersAuth.md)

Create admin user:

```bash
curl -X POST http://localhost:8080/api/auth/bootstrap \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "secure_password",
    "email": "admin@example.com"
  }'
```

Login to obtain JWT token:

```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{
    "username": "admin",
    "password": "secure_password"
  }'
```

#### HMAC Authentication

[Service Account docs](docs/service_accounts.md)

For service-to-service communication, create service accounts and sign requests with:

- `X-Service-ID`: Service account identifier
- `X-Timestamp`: ISO 8601 timestamp
- `X-Signature`: HMAC-SHA256 signature

## TLS/HTTPS Configuration

GoRTMS supports HTTPS with automatic certificate generation for secure communication. The system can operate in both HTTP (development) and HTTPS (production) modes.

### Automatic Certificate Generation

GoRTMS automatically generates self-signed TLS certificates when HTTPS is enabled without providing custom certificates:

```yaml
http:
  enabled: true
  address: "0.0.0.0"
  port: 8080
  tls: true
  # certFile and keyFile left empty for auto-generation
```

On first startup with TLS enabled, GoRTMS will:

1. **Generate RSA 2048-bit certificate** valid for 1 year
2. **Save certificates** to `{dataDir}/tls/server.crt` and `{dataDir}/tls/server.key`
3. **Include common hostnames** (localhost, 127.0.0.1, ::1) in certificate
4. **Log certificate details** for verification

Expected startup output:
```
🔐 TLS certificates generated successfully
Certificate file: ./data/tls/server.crt
Key file: ./data/tls/server.key
Hostname: localhost
Validity: 1 year
Note: Self-signed certificate - browsers will show security warning
HTTPS server listening on https://0.0.0.0:8080
```

### Custom Certificates

For production environments, provide your own certificates:

```yaml
http:
  enabled: true
  address: "0.0.0.0"
  port: 8080
  tls: true
  certFile: "/path/to/your/certificate.pem"
  keyFile: "/path/to/your/private.key"
```

### Security Configuration

Configure HMAC authentication to require TLS for enhanced security:

```yaml
security:
  enableAuthentication: true
  hmac:
    enabled: true
    requireTLS: true  # Reject HMAC requests over HTTP
    timestampWindow: "5m"
```

When `requireTLS` is enabled:
- **HMAC requests over HTTP** are rejected with 404 (security by obscurity)
- **Server logs** explicit security warnings for administrators
- **HTTPS-only enforcement** for service-to-service authentication

### TLS Client Configuration

#### cURL with Self-Signed Certificates

```bash
# Accept self-signed certificates (development)
curl -k https://localhost:8080/api/health

# Or verify against specific certificate
curl --cacert ./data/tls/server.crt https://localhost:8080/api/health
```

#### Application Clients

```go
// Go client accepting self-signed certificates
client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: &tls.Config{
            InsecureSkipVerify: true, // Only for development
        },
    },
}

resp, err := client.Get("https://localhost:8080/api/health")
```

```javascript
// Node.js client with self-signed certificate support
const https = require('https');

const agent = new https.Agent({
    rejectUnauthorized: false // Only for development
});

const response = await fetch('https://localhost:8080/api/health', {
    agent: agent
});
```

### WebSocket over TLS (WSS)

WebSocket connections automatically use WSS when TLS is enabled:

```javascript
// Secure WebSocket connection
const ws = new WebSocket('wss://localhost:8080/api/ws/domains/ecommerce/queues/orders');

// For self-signed certificates in development environments,
// browser security settings may need adjustment
```

### TLS Configuration Reference

| Property | Type | Description | Default |
|----------|------|-------------|---------|
| `http.tls` | boolean | Enable HTTPS/TLS | false |
| `http.certFile` | string | Custom certificate file path | "" (auto-generate) |
| `http.keyFile` | string | Custom private key file path | "" (auto-generate) |
| `security.hmac.requireTLS` | boolean | Force HMAC over HTTPS only | false |

### Production Deployment

#### Let's Encrypt Integration

For public-facing deployments, use Let's Encrypt certificates:

```bash
# Obtain Let's Encrypt certificate
certbot certonly --standalone -d yourdomain.com

# Configure GoRTMS
```

```yaml
http:
  tls: true
  certFile: "/etc/letsencrypt/live/yourdomain.com/fullchain.pem"
  keyFile: "/etc/letsencrypt/live/yourdomain.com/privkey.pem"
```

#### Corporate PKI

For enterprise environments with internal Certificate Authority:

```yaml
http:
  tls: true
  certFile: "/etc/ssl/certs/gortms.crt"
  keyFile: "/etc/ssl/private/gortms.key"
```

### Security Best Practices

1. **Always enable TLS in production** environments
2. **Set `requireTLS: true`** for HMAC authentication
3. **Use proper certificates** from trusted CA for public deployment
4. **Regularly rotate certificates** before expiration
5. **Monitor certificate validity** in production systems
6. **Configure firewalls** to block HTTP when HTTPS is available

## API Usage Examples

### Domain and Queue Management

```bash
# Create domain (with authentication)
curl -X POST http://localhost:8080/api/domains \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "name": "ecommerce",
    "schema": {
      "fields": {
        "order_id": "string",
        "amount": "number",
        "timestamp": "string"
      }
    }
  }'

# Create queue with advanced configuration
curl -X POST http://localhost:8080/api/domains/ecommerce/queues \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "name": "orders",
    "config": {
      "isPersistent": true,
      "maxSize": 10000,
      "ttl": "24h",
      "workerCount": 4,
      "retryEnabled": true,
      "retryConfig": {
        "maxRetries": 3,
        "initialDelay": "1s",
        "factor": 2.0,
        "maxDelay": "30s"
      },
      "circuitBreakerEnabled": true,
      "circuitBreakerConfig": {
        "errorThreshold": 0.5,
        "successThreshold": 5,
        "minimumRequests": 10,
        "openTimeout": "30s"
      }
    }
  }'
```

### Consumer Group Operations

```bash
# Create consumer group
curl -X POST http://localhost:8080/api/domains/ecommerce/queues/orders/consumer-groups \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "groupId": "order-processors",
    "ttl": "24h"
  }'

# Consume messages (pull-based)
curl -X GET "http://localhost:8080/api/domains/ecommerce/queues/orders/consumer-groups/order-processors/messages?count=5&timeout=30" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# Get consumer group status
curl -X GET http://localhost:8080/api/domains/ecommerce/queues/orders/consumer-groups/order-processors \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Message Publishing and Consumption

```bash
# Publish message
curl -X POST http://localhost:8080/api/domains/ecommerce/queues/orders/messages \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{
    "order_id": "ord_12345",
    "amount": 99.99,
    "timestamp": "2025-06-29T10:30:00Z"
  }'

# Long polling for messages
curl -X GET "http://localhost:8080/api/domains/ecommerce/queues/orders/messages?timeout=30&max=10&group=processors" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

### Position Management and Replay

```bash
# Reset consumer group position for replay
curl -X PUT http://localhost:8080/api/domains/ecommerce/queues/orders/consumer-groups/order-processors \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN" \
  -d '{"position": 12345}'
```

## Real-Time Message Flow Visibility

Connect to WebSocket endpoint for live message observation:

```javascript
// Direct WebSocket connection for message flow visibility
const ws = new WebSocket('ws://localhost:8080/api/ws/domains/ecommerce/queues/orders');

ws.onmessage = (event) => {
  const data = JSON.parse(event.data);
  
  switch(data.type) {
    case 'connected':
      console.log('Connected to queue:', data.queue);
      console.log('Subscription ID:', data.subscriptionId);
      break;
      
    case 'message':
      console.log('New message:', data.payload);
      console.log('Message ID:', data.id);
      break;
      
    case 'pong':
      console.log('Server responded to ping');
      break;
  }
};

// Keep connection alive
ws.send(JSON.stringify({ type: 'ping' }));

// Publish via WebSocket
ws.send(JSON.stringify({
  type: 'publish',
  payload: {
    order_id: 'ord_123',
    amount: 99.99
  }
}));
```

## System Monitoring

GoRTMS provides comprehensive monitoring through dedicated metrics endpoints:

```bash
# Global system statistics
curl -X GET "http://localhost:8080/api/stats?period=1h&granularity=5m" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# Current resource utilization
curl -X GET "http://localhost:8080/api/resources/current" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# Resource usage history
curl -X GET "http://localhost:8080/api/resources/history?limit=100" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"

# Domain-specific metrics
curl -X GET "http://localhost:8080/api/resources/domains/ecommerce" \
  -H "Authorization: Bearer YOUR_JWT_TOKEN"
```

## Configuration Reference

### Queue Configuration

| Property | Type | Description | Default |
|----------|------|-------------|---------|
| `isPersistent` | boolean | Enable message persistence | true |
| `maxSize` | int | Maximum queue buffer size | 1000 |
| `ttl` | string | Message time-to-live | "24h" |
| `workerCount` | int | Parallel processing workers | 2 |

### Retry Configuration

| Property | Type | Description | Default |
|----------|------|-------------|---------|
| `retryEnabled` | boolean | Enable retry mechanism | false |
| `maxRetries` | int | Maximum retry attempts | 3 |
| `initialDelay` | string | Initial retry delay | "1s" |
| `factor` | float | Exponential backoff factor | 2.0 |
| `maxDelay` | string | Maximum retry delay | "30s" |

### Circuit Breaker Configuration

| Property | Type | Description | Default |
|----------|------|-------------|---------|
| `circuitBreakerEnabled` | boolean | Enable circuit breaker | false |
| `errorThreshold` | float | Error rate threshold (0-1) | 0.5 |
| `successThreshold` | int | Successes to close circuit | 5 |
| `minimumRequests` | int | Min requests before evaluation | 10 |
| `openTimeout` | string | Circuit open duration | "30s" |

## Use Cases

### Event Sourcing Systems
Position-based replay enables complete event stream reconstruction. Consumer groups maintain independent projections from the same event stream.

### Microservice Communication
Circuit breaker pattern prevents cascade failures. Retry mechanisms handle transient network issues. Consumer groups provide reliable message delivery guarantees.

### Real-Time Message Visibility
WebSocket observation provides immediate visibility into data flows. Position tracking enables historical data analysis and replay scenarios.

### Background Job Processing
Worker pools control resource utilization. Retry mechanisms handle processing failures gracefully. Consumer groups enable horizontal scaling of job processors.

## API Reference

### Core Resources

- **Domains**: `/api/domains`
- **Queues**: `/api/domains/{domain}/queues`
- **Messages**: `/api/domains/{domain}/queues/{queue}/messages`
- **Consumer Groups**: `/api/domains/{domain}/queues/{queue}/consumer-groups`

### Monitoring and Observability

- **System Monitoring**: `/api/stats`, `/api/resources/*`
- **Message Flow Visibility**: `/api/ws/domains/{domain}/queues/{queue}`
- **Health Check**: `/api/health`

### Authentication

- **Login**: `/api/auth/login`
- **Bootstrap**: `/api/auth/bootstrap`

## Architecture

GoRTMS follows hexagonal architecture with clear separation between:

```
domain/          # Core business logic
├── model/       # Domain entities
├── service/     # Business services
└── port/        # Interfaces
    ├── inbound/ # Service interfaces
    └── outbound/# Repository interfaces

adapter/         # Infrastructure adapters
├── inbound/     # API adapters
│   ├── rest/    # HTTP REST API
│   └── websocket/# WebSocket handler
└── outbound/    # Storage adapters
    ├── storage/ # Message/domain persistence
    └── subscription/# Subscription registry
```

## Development

### Configuration Changes

Modify `config.yaml` and restart the server to apply configuration changes.

### Logs and Debugging

Set log level to debug for detailed logging:

```yaml
general:
  logLevel: "debug"
```

## Performance Characteristics

### Horizontal Scaling
Consumer groups automatically distribute messages among active consumers without requiring external coordination. Adding consumers to existing groups increases throughput proportionally.

### Vertical Scaling
Queue worker count controls parallel processing within individual queues. Buffer sizes control memory usage versus throughput trade-offs.

### Memory Management
The system uses bounded channels with configurable sizes. Circuit breakers prevent memory exhaustion during failure scenarios. TTL-based cleanup prevents resource leaks from abandoned consumer groups.

## License

This project is available under the MIT License.