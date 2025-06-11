# GoRTMS Authentication & User Management API

## Overview

GoRTMS provides a comprehensive authentication and user management system with JWT-based security, role-based access control, and secure user storage.

**Security Features:**
- JWT Bearer token authentication
- Role-based access control (Admin, User)
- Encrypted user storage with machine-specific keys
- Argon2 password hashing with individual salts
- Auto-bootstrap admin creation

**Configuration:**
Authentication can be enabled/disabled via `config.yaml`:
```yaml
security:
  enableAuthentication: true  # Set to false to disable auth
  enableAuthorization: true
```

---

## Authentication Endpoints

### 🔐 Login
Authenticate a user and receive a JWT token.

**Endpoint:** `POST /api/auth/login`  
**Access:** Public  
**Content-Type:** `application/json`

**Request Body:**
```json
{
  "username": "admin",
  "password": "admin"
}
```

**Success Response (200):**
```json
{
  "user": {
    "id": "uuid-string",
    "username": "admin",
    "role": "admin",
    "createdAt": "2025-06-11T10:30:00Z",
    "lastLogin": "2025-06-11T10:30:00Z",
    "enabled": true
  },
  "token": "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
}
```

**Error Responses:**
- `400 Bad Request` - Missing username/password
- `401 Unauthorized` - Invalid credentials
- `401 Unauthorized` - User disabled

**Example:**
```bash
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'
```

---

### 🚀 Bootstrap Admin
Create the first admin user if no users exist. Returns a random secure password.

**Endpoint:** `POST /api/auth/bootstrap`  
**Access:** Public  
**Content-Type:** `application/json`

**Request Body:** Empty

**Success Response (200):**
```json
{
  "admin": {
    "id": "uuid-string",
    "username": "admin",
    "role": "admin",
    "createdAt": "2025-06-11T10:30:00Z",
    "enabled": true
  },
  "password": "Kx9mP2!vN8qR5@Wy",
  "message": "Admin account created with random password. Save this password - it will not be shown again!"
}
```

**Error Responses:**
- `409 Conflict` - Users already exist
- `500 Internal Server Error` - Database error

**Example:**
```bash
curl -X POST http://localhost:8080/api/auth/bootstrap
```

**Note:** This endpoint is mainly for emergency recovery. The system auto-creates `admin/admin` on first startup.

---

### 👤 Get Profile
Get the profile of the currently authenticated user.

**Endpoint:** `GET /api/auth/profile`  
**Access:** Authenticated users  
**Authorization:** `Bearer <jwt-token>`

**Request Body:** None

**Success Response (200):**
```json
{
  "id": "uuid-string",
  "username": "admin",
  "role": "admin",
  "createdAt": "2025-06-11T10:30:00Z",
  "lastLogin": "2025-06-11T10:30:00Z",
  "enabled": true
}
```

**Error Responses:**
- `401 Unauthorized` - Missing/invalid token
- `500 Internal Server Error` - User not found in context

**Example:**
```bash
curl -X GET http://localhost:8080/api/auth/profile \
  -H "Authorization: Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9..."
```

---

## User Management Endpoints (Admin Only)

### 👥 Create User
Create a new user account. Only admins can create users.

**Endpoint:** `POST /api/admin/users`  
**Access:** Admin only  
**Authorization:** `Bearer <admin-jwt-token>`  
**Content-Type:** `application/json`

**Request Body:**
```json
{
  "username": "newuser",
  "password": "securepassword",
  "role": "user"  // Optional, defaults to "user"
}
```

**Available Roles:**
- `admin` - Full access, can manage users
- `user` - Standard access, cannot manage users

**Success Response (200):**
```json
{
  "id": "uuid-string",
  "username": "newuser",
  "role": "user",
  "createdAt": "2025-06-11T10:30:00Z",
  "enabled": true
}
```

**Error Responses:**
- `400 Bad Request` - Missing username/password
- `400 Bad Request` - User already exists
- `401 Unauthorized` - Missing/invalid token
- `403 Forbidden` - Non-admin user

**Example:**
```bash
curl -X POST http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer <admin-token>" \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"secure123","role":"user"}'
```

---

### 📋 List Users
Get a list of all users in the system.

**Endpoint:** `GET /api/admin/users`  
**Access:** Admin only  
**Authorization:** `Bearer <admin-jwt-token>`

**Request Body:** None

**Success Response (200):**
```json
[
  {
    "id": "admin-uuid",
    "username": "admin",
    "role": "admin",
    "createdAt": "2025-06-11T10:00:00Z",
    "lastLogin": "2025-06-11T10:30:00Z",
    "enabled": true
  },
  {
    "id": "user-uuid",
    "username": "alice",
    "role": "user",
    "createdAt": "2025-06-11T10:15:00Z",
    "lastLogin": "2025-06-11T10:25:00Z",
    "enabled": true
  }
]
```

**Error Responses:**
- `401 Unauthorized` - Missing/invalid token
- `403 Forbidden` - Non-admin user
- `500 Internal Server Error` - Database error

**Example:**
```bash
curl -X GET http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer <admin-token>"
```

---

## Authentication Flow

### 1. System Startup
```
┌─────────────────────────────────────────────────┐
│ System starts → Check for existing users       │
│                                                 │
│ IF no users exist:                              │
│   → Auto-create admin/admin                     │
│   → Log: "Default admin created"                │
│                                                 │
│ ELSE:                                           │
│   → Log: "Users exist, skipping bootstrap"     │
└─────────────────────────────────────────────────┘
```

### 2. User Authentication
```
┌─────────────────────────────────────────────────┐
│ 1. POST /api/auth/login                         │
│    {"username":"admin", "password":"admin"}     │
│                                                 │
│ 2. Receive JWT token                            │
│    {"token": "eyJ...", "user": {...}}          │
│                                                 │
│ 3. Include token in subsequent requests         │
│    Authorization: Bearer eyJ...                 │
│                                                 │
│ 4. Token validated by middleware                │
│    → User context added to request              │
└─────────────────────────────────────────────────┘
```

### 3. Admin User Management
```
┌─────────────────────────────────────────────────┐
│ 1. Admin logs in → Receives admin JWT           │
│                                                 │
│ 2. Create users via POST /api/admin/users       │
│    → Specify username, password, role           │
│                                                 │
│ 3. List users via GET /api/admin/users          │
│    → View all users in system                   │
│                                                 │
│ 4. Users can login with their credentials       │
│    → Receive user-level JWT tokens              │
└─────────────────────────────────────────────────┘
```

---

## Security Model

### JWT Token Structure
```json
{
  "username": "admin",
  "role": "admin",
  "exp": 1640995200,
  "iat": 1640991600
}
```

### Role-Based Access Control

| Endpoint | Public | User | Admin |
|----------|--------|------|-------|
| `POST /api/auth/login` | ✅ | ✅ | ✅ |
| `POST /api/auth/bootstrap` | ✅ | ✅ | ✅ |
| `GET /api/auth/profile` | ❌ | ✅ | ✅ |
| `POST /api/admin/users` | ❌ | ❌ | ✅ |
| `GET /api/admin/users` | ❌ | ❌ | ✅ |
| All other GoRTMS APIs | ❌* | ✅ | ✅ |

*\* When authentication is enabled*

### Protected Routes
When authentication is enabled, all routes except the following require a valid JWT token:
- `/api/auth/login`
- `/api/auth/bootstrap`
- `/api/health`
- `/web/*` (static assets)
- `/` (root)

### Password Security
- **Hashing:** Argon2id with individual salts
- **Storage:** Encrypted with machine-specific keys
- **Transport:** HTTPS recommended for production

---

## Error Responses

All endpoints return errors in this format:

```json
{
  "error": "error_code",
  "message": "Human readable error message"
}
```

### Common Error Codes
- `unauthorized` - Missing or invalid authentication
- `forbidden` - Insufficient permissions
- `bootstrap_not_needed` - Bootstrap called when users exist
- `validation_error` - Invalid request data

---

## Configuration

### Enable/Disable Authentication

**In `config.yaml`:**
```yaml
security:
  enableAuthentication: true   # Enable JWT auth middleware
  enableAuthorization: true    # Enable role-based access control
  adminUsername: admin         # Default admin username
  adminPassword: admin         # Default admin password (auto-bootstrap)
```

### JWT Configuration

```yaml
http:
  jwt:
    secret: "your-secret-key"     # Change in production!
    expirationMinutes: 60         # Token validity duration
```

**Important:** Change the JWT secret in production environments!

---

## Examples

### Complete Authentication Workflow

```bash
# 1. Login as default admin
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"admin","password":"admin"}'

# Response: {"user":{...},"token":"eyJ..."}

# 2. Create a new user (using admin token)
curl -X POST http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer eyJ..." \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"secure123","role":"user"}'

# 3. List all users
curl -X GET http://localhost:8080/api/admin/users \
  -H "Authorization: Bearer eyJ..."

# 4. User logs in with their credentials
curl -X POST http://localhost:8080/api/auth/login \
  -H "Content-Type: application/json" \
  -d '{"username":"alice","password":"secure123"}'

# 5. User accesses their profile
curl -X GET http://localhost:8080/api/auth/profile \
  -H "Authorization: Bearer <user-token>"
```

### Frontend Integration

```javascript
// Login and store token
const login = async (username, password) => {
  const response = await fetch('/api/auth/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password })
  });
  
  if (response.ok) {
    const { user, token } = await response.json();
    localStorage.setItem('authToken', token);
    return { user, token };
  }
  
  throw new Error('Login failed');
};

// Make authenticated requests
const apiCall = async (url, options = {}) => {
  const token = localStorage.getItem('authToken');
  
  return fetch(url, {
    ...options,
    headers: {
      'Authorization': `Bearer ${token}`,
      'Content-Type': 'application/json',
      ...options.headers
    }
  });
};

// Create user (admin only)
const createUser = async (userData) => {
  const response = await apiCall('/api/admin/users', {
    method: 'POST',
    body: JSON.stringify(userData)
  });
  
  if (!response.ok) {
    throw new Error('Failed to create user');
  }
  
  return response.json();
};
```

---

## Notes

- **Default Credentials:** `admin/admin` (change after first login)
- **Token Expiration:** Configurable, default 60 minutes
- **Security:** Encrypted storage, secure password hashing
- **Scalability:** Stateless JWT tokens, no server-side sessions
- **Flexibility:** Authentication can be completely disabled
