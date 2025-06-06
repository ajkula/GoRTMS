# Complete Guide to Getting Started and Testing GoRTMS

**GO Real-Time Messaging System**

## 1. Environment Setup

### Backend Installation

```bash
# Clone the repository (or create the folder structure)
git clone https://github.com/ajkula/GoRTMS.git
cd GoRTMS

# Install dependencies
go mod tidy
```

### Frontend Installation

```bash
# Go to the web folder
cd web

# Install Node.js dependencies
npm install

# Build the interface for production
npm run build
```

## 2. Configuration

### Generate Default Configuration

```bash
# From the project root
go run cmd/server/main.go --generate-config
```

This will create a `config.yaml` file that you can customize to your needs.

### Protobuf Configuration (if using gRPC)

```bash
# On Windows, run the PowerShell script
.\setup-proto.ps1

# Or manually
protoc --go_out=. --go-grpc_out=. adapter/inbound/grpc/proto/realtimedb.proto
```

## 3. Build and Start

### Build the Application

```bash
# Compile the application
go build -o gortms cmd/server/main.go
```

### Start the Server

```bash
# Run the application
./gortms

# Or with a specific config
./gortms --config=my-config.yaml
```

The server should start and you'll see logs like:
```
Starting GoRTMS...
Node ID: node1
Data directory: ./data
HTTP server listening on 0.0.0.0:8080
GoRTMS started successfully
```

## 4. Access the User Interface

Open your web browser and go to:
```
http://localhost:8080/ui/
```

You should see the GoRTMS admin interface with the dashboard, domain management, and queue monitor.

## 5. Feature Testing

### Create a Test Domain

1. In the web interface, click on "Domains" in the sidebar
2. Click "Create Domain"
3. Fill in the details:
   - Name: `test`
   - Schema:
     ```json
     {
       "fields": {
         "content": "string",
         "priority": "number"
       }
     }
     ```
4. Click "Create"

### Create a Queue

1. Click on the created domain
2. Click "Create Queue"
3. Fill in the details:
   - Name: `messages`
   - Configuration:
     ```json
     {
       "isPersistent": true,
       "maxSize": 1000,
       "ttl": 86400000,
       "deliveryMode": "broadcast"
     }
     ```
4. Click "Create"

### Test the Real-Time Monitor

1. Open the queue monitor
2. Open a new terminal window
3. Send a test message using curl:

```bash
curl -X POST http://localhost:8080/api/domains/test/queues/messages/messages   -H "Content-Type: application/json"   -d '{"content": "Hello, GoRTMS!", "priority": 1}'
```

4. Watch the message appear in the real-time monitor

## 6. Continuous Development

To work on the frontend during development:

```bash
cd web
npm run dev
```

This will start a development server on port 3000 with hot reload, making it easier to update the interface.

To stop the server, press `Ctrl+C` in the terminal.

## 7. Troubleshooting

### Common Issues

- **Port already in use**: Change the port in `config.yaml`
- **Data folder access issues**: Check permissions on the `data` folder
- **gRPC compilation errors**: Make sure protobuf generation was successful
- **Web interface not available**: Make sure you built the frontend with `npm run build`

### Logs and Debugging

For more detailed logs, change the log level in `config.yaml`:

```yaml
general:
  logLevel: "debug"
```

## 8. Next Steps

Once your GoRTMS instance is running:

1. Implement missing domain services
2. Add persistent storage (file adapter or database)
3. Configure additional protocol adapters (AMQP, MQTT)
4. Develop automated tests for reliability

Your system is now ready for thorough testing and further feature development!



# GoRTMS RESTful API

## Authentication
- `POST /api/auth/login` – Obtain a JWT token
- `POST /api/auth/refresh` – Refresh a JWT token

## Domains (Schemas)
- `GET /api/domains` – List all domains
- `POST /api/domains` – Create a new domain
- `GET /api/domains/{domain}` – Get domain details
- `PUT /api/domains/{domain}` – Update a domain
- `DELETE /api/domains/{domain}` – Delete a domain

## Queues
- `GET /api/domains/{domain}/queues` – List all queues in a domain
- `POST /api/domains/{domain}/queues` – Create a new queue
- `GET /api/domains/{domain}/queues/{queue}` – Get queue details
- `PUT /api/domains/{domain}/queues/{queue}` – Update a queue
- `DELETE /api/domains/{domain}/queues/{queue}` – Delete a queue

## Messages
- `POST /api/domains/{domain}/queues/{queue}/messages` – Publish a message
- `GET /api/domains/{domain}/queues/{queue}/messages` – Retrieve messages (long polling)
- `DELETE /api/domains/{domain}/queues/{queue}/messages/{messageId}` – Acknowledge a message

## Routing Rules
- `GET /api/domains/{domain}/routes` – List all routing rules
- `POST /api/domains/{domain}/routes` – Create a new routing rule
- `DELETE /api/domains/{domain}/routes/{sourceQueue}/{destQueue}` – Delete a routing rule

## WebSockets
- `WS /api/ws/domains/{domain}/queues/{queue}` – Subscribe to queue messages

## Monitoring
- `GET /api/stats` – Get global statistics
- `GET /api/domains/{domain}/stats` – Get statistics for a domain
- `GET /api/domains/{domain}/queues/{queue}/stats` – Get statistics for a queue

## Administration
- `GET /api/admin/users` – List all users
- `POST /api/admin/users` – Create a new user
- `PUT /api/admin/users/{user}` – Update a user
- `DELETE /api/admin/users/{user}` – Delete a user
- `POST /api/admin/backup` – Create a backup
- `GET /api/admin/backup` – List backups
- `POST /api/admin/restore` – Restore from backup

```
GoRTMS/
├── domain/                # Core business logic
│   ├── model/             # Domain entities and value objects
│   │   ├── message.go     # Message model
│   │   ├── queue.go       # Queue model
│   │   ├── domain.go      # Domain (schema) model
│   │   └── routing.go     # Routing rules model
│   ├── service/           # Business services implementing logic
│   │   ├── message_service.go
│   │   ├── domain_service.go
│   │   ├── queue_service.go
│   │   └── routing_service.go
│   └── port/              # Ports (interfaces) for external interaction
│       ├── inbound/       # Inbound ports for adapters
│       │   ├── message_service.go
│       │   ├── domain_service.go
│       │   ├── queue_service.go
│       │   └── routing_service.go
│       └── outbound/      # Outbound ports for adapters
│           ├── message_repository.go
│           ├── domain_repository.go
│           └── subscription_registry.go
├── adapter/               # Adapters implementing the ports
│   ├── inbound/           # Inbound adapters (APIs)
│   │   ├── rest/          # REST API
│   │   │   └── handler.go
│   │   ├── websocket/     # WebSocket
│   │   │   └── handler.go
│   │   ├── amqp/          # AMQP (RabbitMQ)
│   │   │   └── server.go
│   │   ├── mqtt/          # MQTT
│   │   │   └── server.go
│   │   ├── grpc/          # gRPC (to be added later)
│   │   │   ├── proto/
│   │   │   └── server.go
│   │   └── graphql/       # GraphQL (to be added later)
│   │       ├── schema/
│   │       └── resolver.go
│   └── outbound/          # Outbound adapters (storage, etc.)
│       ├── storage/
│       │   ├── memory/    # In-memory storage
│       │   │   ├── message_repository.go
│       │   │   └── domain_repository.go
│       │   ├── file/      # File-based storage
│       │   └── database/  # Database storage
│       └── subscription/
│           └── memory/    # In-memory subscription management
├── config/                # Configuration
├── cmd/                   # Entry points
│   └── server/            # Main server
│       └── main.go
└── web/                   # Web interface for monitoring
```

# GoRTMS Frontend Installation and Usage Guide

This guide explains how to install and configure the React frontend for your GoRTMS application.

## Prerequisites

- Node.js 16.x or higher
- npm 8.x or higher (or yarn)
- Go 1.16 or higher (for the backend)

## Folder Structure

The frontend is located in the `web` directory at the root of your project:

```
GoRTMS/
├── web/
│   ├── public/         # Static files
│   ├── src/            # React source code
│   │   ├── components/ # React components
│   │   ├── pages/      # Application pages
│   │   ├── styles/     # CSS files
│   │   ├── App.js      # Main component
│   │   └── index.js    # Entry point
│   ├── package.json    # npm dependencies
│   ├── webpack.config.js # Webpack configuration
│   └── tailwind.config.js # Tailwind configuration
└── ...                 # Other project files
```

## Installation

1. **Navigate to the web folder:**
   ```bash
   cd web
   ```

2. **Install dependencies:**
   ```bash
   npm install
   # or with yarn
   yarn install
   ```

## Available Scripts

- **Development with auto-reload:**
  
  ```bash
  npm run dev
  # or with yarn
  yarn dev
  ```
This starts a development server on [http://localhost:3000](http://localhost:3000) with hot reloading.
  
- **Production build:**
  ```bash
  npm run build
  # or with yarn
  yarn build
  ```
  This generates optimized files in the `dist` folder.

- **Build with file watch:**
  ```bash
  npm run watch
  # or with yarn
  yarn watch
  ```
  Useful for development while watching real-time updates served by the Go backend.

## Integration with the Backend

### Option 1: Serve static files via Go

After building the frontend with `npm run build`, the generated files in the `dist` folder can be served by your Go server:

```go
// In your main.go or equivalent
router.PathPrefix("/ui/").Handler(http.StripPrefix("/ui/", http.FileServer(http.Dir("./web/dist"))))
```

### Option 2: Development with proxy

During development, you can run the webpack dev server with a proxy to your backend:

1. Ensure your GoRTMS backend is running on port 8080
2. Start the frontend development server: `npm run dev`
3. The proxy configured in `webpack.config.js` will automatically redirect `/api/*` requests to your backend

## React File Organization

- **components/** – Reusable components like buttons, cards, etc.
- **pages/** – Main screens of the application
- **styles/** – CSS files, primarily for Tailwind configuration
- **App.js** – Main component handling routing and layout
- **index.js** – Entry point mounting the app to the DOM

## Customization

### Theme and Colors

You can customize the colors and theme by editing the `tailwind.config.js` file.

### Logo and Branding

Replace the logo in `public/images/logo.svg` with your own logo.

### Home Page

Edit the homepage (Dashboard) content by modifying `src/pages/Dashboard.js`.

## Production Deployment

To deploy in production:

1. Build the frontend:

   ```bash
   npm run build
   ```

2. Copy the contents of the `dist` folder to a directory accessible by your Go server, or configure the server to serve directly from that folder.

3. Ensure that the path settings in your Go configuration are correct.

## Common Issues and Troubleshooting

### Code Changes Not Visible

* Ensure that webpack is running in watch mode (`npm run watch`)
* Clear your browser cache
* Check the browser console for errors

### Webpack Build Errors

* Make sure all dependencies are installed (`npm install`)
* Look at the specific errors in the webpack output
* Try deleting `node_modules` and reinstalling:

  ```bash
  rm -rf node_modules && npm install
  ```

### API Issues

* Verify that API requests use the correct paths
* Confirm that the backend server is running
* Inspect the API response in the browser DevTools Network tab

## Additional Resources

* [React Documentation](https://reactjs.org/docs/getting-started.html)
* [Tailwind CSS Documentation](https://tailwindcss.com/docs)
* [Webpack Documentation](https://webpack.js.org/concepts/)
* [Recharts (Charts Library)](https://recharts.org/en-US/)
* [Lucide React (Icons)](https://lucide.dev/guide/packages/lucide-react)

```
web/
├── assets/
│   ├── css/
│   │   └── tailwind.css
│   ├── js/
│   │   ├── api.js          # API client to communicate with the backend
│   │   └── utils.js        # Utility functions
│   └── images/
│       └── logo.svg
├── components/
│   ├── layout/
│   │   ├── Sidebar.js      # Sidebar navigation
│   │   ├── Header.js       # Header with breadcrumbs and actions
│   │   └── Layout.js       # Main layout
│   ├── common/
│   │   ├── Button.js       # Reusable buttons
│   │   ├── Card.js         # Card component
│   │   ├── Modal.js        # Modal window
│   │   └── Table.js        # Data table
│   ├── domain/
│   │   ├── DomainList.js   # Domain list
│   │   ├── DomainForm.js   # Creation/edit form
│   │   └── SchemaEditor.js # Schema editor
│   ├── queue/
│   │   ├── QueueList.js    # Queue list
│   │   ├── QueueForm.js    # Creation/edit form
│   │   └── QueueMonitor.js # Real-time message monitor
│   ├── message/
│   │   ├── MessageList.js  # Message list
│   │   ├── MessageForm.js  # Creation form
│   │   └── MessageView.js  # Detailed message view
│   └── routing/
│       ├── RuleList.js     # Routing rules list
│       ├── RuleForm.js     # Creation/edit form
│       └── RuleVisualizer.js # Rule visualization
├── pages/
│   ├── Dashboard.js        # Main dashboard
│   ├── Domains.js          # Domain management
│   ├── Queues.js           # Queue management
│   ├── Messages.js         # Message viewer
│   ├── Routes.js           # Routing rules management
│   └── Settings.js         # System settings
├── app.js                  # Application entry point
└── index.html              # Main HTML page
```
