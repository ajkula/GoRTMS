package main

import (
	"context"
	"crypto/tls"
	"embed"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/gorilla/mux"

	"github.com/ajkula/GoRTMS/adapter/inbound/grpc"
	"github.com/ajkula/GoRTMS/adapter/inbound/rest"
	"github.com/ajkula/GoRTMS/adapter/inbound/websocket"
	"github.com/ajkula/GoRTMS/adapter/outbound/crypto"
	"github.com/ajkula/GoRTMS/adapter/outbound/filewatcher"
	"github.com/ajkula/GoRTMS/adapter/outbound/logging"
	"github.com/ajkula/GoRTMS/adapter/outbound/machineid"
	"github.com/ajkula/GoRTMS/adapter/outbound/storage"
	"github.com/ajkula/GoRTMS/adapter/outbound/storage/memory"
	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/service"

	// Temporary imports for compilation
	"github.com/ajkula/GoRTMS/domain/port/inbound"
	"github.com/ajkula/GoRTMS/domain/port/outbound"
)

//go:embed index.html
//go:embed bundle.js
//go:embed favicon.ico
var uiFiles embed.FS

func main() {
	// Handle command-line arguments
	var configPath string
	var generateConfig bool
	var showVersion bool

	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.BoolVar(&generateConfig, "generate-config", false, "Generate default configuration file")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.Parse()

	// Display version information
	if showVersion {
		fmt.Println("GoRTMS Version 1.0.0")
		os.Exit(0)
	}

	// Generate a default configuration file
	if generateConfig {
		cfg := config.DefaultConfig()
		cfg.ConfigPath = configPath
		err := config.SaveConfig(cfg, configPath)
		if err != nil {
			fmt.Printf("Error generating config file: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Default configuration file generated at: %s\n", configPath)
		os.Exit(0)
	}

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Initialize structured logger
	logger := logging.NewSlogAdapter(cfg)

	logger.Info("Starting GoRTMS...")
	logger.Info("Node ID", "nodeID", cfg.General.NodeID)
	logger.Info("Data directory", "dataDir", cfg.General.DataDir)

	// Create the data directory if it doesn't exist
	if err := os.MkdirAll(cfg.General.DataDir, 0755); err != nil {
		logger.Error("Failed to create data directory", "ERROR", err)
	}

	// Create a cancellable context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Dedicated pprof server on port 6060
	go func() {
		logger.Info("Starting pprof server on port", "port", cfg.HTTP.Port)
		err := http.ListenAndServe("localhost:6060", nil)
		logger.Error("Error starting pprof server", "ERROR", err)
	}()

	// Initialize repositories (outgoing adapters)
	messageRepo := memory.NewMessageRepository(logger)
	domainRepo := memory.NewDomainRepository(logger)
	consumerGroupRepo := memory.NewConsumerGroupRepository(logger, messageRepo)
	subscriptionReg := memory.NewSubscriptionRegistry()

	// Create services (domain implementations)
	statsService := service.NewStatsService(ctx, logger, domainRepo, messageRepo)
	queueService := service.NewQueueService(ctx, logger, domainRepo, statsService)
	messageService := service.NewMessageService(
		ctx,
		logger,
		domainRepo,
		messageRepo,
		consumerGroupRepo,
		subscriptionReg,
		queueService,
		statsService,
	)

	// Inject messageService into queueService
	if queueSvc, ok := queueService.(*service.QueueServiceImpl); ok {
		queueSvc.SetMessageService(messageService)
	}

	domainService := service.NewDomainService(domainRepo, queueService, ctx)
	routingService := service.NewRoutingService(domainRepo, ctx)

	// Initialize the ConsumerGroupService
	consumerGroupService := service.NewConsumerGroupService(
		ctx,
		logger,
		consumerGroupRepo,
		messageRepo,
	)

	// Initialize the resource monitoring service
	resourceMonitorService := service.NewResourceMonitorService(
		domainRepo,
		messageRepo,
		queueService,
		ctx,
	)

	// Initialize crypto services
	machineIDService := machineid.NewHardwareMachineID()
	cryptoService := crypto.NewAESCryptoService()

	// Initialize user repository with secure storage
	userRepoPath := filepath.Join(cfg.General.DataDir, "users.db")
	userRepo, err := storage.NewSecureUserRepository(
		userRepoPath,
		cryptoService,
		machineIDService,
		logger,
	)
	if err != nil {
		logger.Error("Failed to initialize user repository", "error", err)
		return
	}

	serviceRepoPath := filepath.Join(cfg.General.DataDir, "service.db")
	serviceRepo, err := storage.NewSecureServiceRepository(serviceRepoPath, logger)
	if err != nil {
		logger.Error("Failed to create service repository", "error", err)
		os.Exit(1)
	}

	// Initialize the auth service
	authService := service.NewAuthService(
		userRepo,
		cryptoService,
		logger,
		cfg.HTTP.JWT.Secret,
		cfg.HTTP.JWT.ExpirationMinutes,
	)

	if err := autoBootstrapAdmin(authService, logger); err != nil {
		logger.Error("Failed to auto-bootstrap admin", "error", err)
	}

	if err := domainRepo.StoreDomain(ctx, &model.Domain{
		Name: "SYSTEM",
		Queues: map[string]*model.Queue{
			"_account_requests": {
				Name:       "_account_requests",
				DomainName: "SYSTEM",
				Config: model.QueueConfig{
					IsPersistent: true,
					MaxSize:      1000,
					TTL:          0,
					WorkerCount:  2,
					RetryEnabled: true,
					RetryConfig: &model.RetryConfig{
						MaxRetries:   3,
						InitialDelay: time.Second,
						MaxDelay:     time.Hour,
						Factor:       0.3,
					},
					CircuitBreakerEnabled: true,
					CircuitBreakerConfig: &model.CircuitBreakerConfig{
						SuccessThreshold: 1,
						ErrorThreshold:   3,
						MinimumRequests:  3,
						OpenTimeout:      time.Duration(5 * time.Minute),
					},
				},
			},
		},
		System: true,
	}); err != nil {
		log.Fatal("Could not create system domain")
	}

	// Initialize account request repository
	accountRequestRepoPath := filepath.Join(cfg.General.DataDir, "account_requests.db")
	accountRequestRepo, err := storage.NewSecureAccountRequestRepository(
		accountRequestRepoPath,
		cryptoService,
		machineIDService,
		logger,
	)
	if err != nil {
		logger.Error("Failed to create account request repository", "error", err)
		os.Exit(1)
	}

	// Initialize account request service
	accountRequestService := service.NewAccountRequestService(
		accountRequestRepo,
		userRepo,
		cryptoService,
		messageService,
		authService,
		logger,
	)

	// Initialize file watcher service
	fileWatcher, err := filewatcher.NewFSWatcher()
	if err != nil {
		logger.Error("Failed to create file watcher", "error", err)
		os.Exit(1)
	}

	fileWatcherService := service.NewFileWatcherService(
		fileWatcher,
		accountRequestService,
		logger,
	)

	// Start file watcher service
	if err := fileWatcherService.Start(ctx); err != nil {
		logger.Error("Failed to start file watcher service", "error", err)
		os.Exit(1)
	}

	// Watch account request file
	if err := fileWatcherService.WatchAccountRequestFile(ctx, userRepoPath); err != nil {
		logger.Error("Failed to watch account request file", "error", err)
	}

	// Create HTTP router
	router := mux.NewRouter()

	// Configure the incoming adapters
	if cfg.HTTP.Enabled {
		// Ensure TLS certificates exist if TLS is enabled
		if err := config.EnsureTLSCertificates(cfg, cryptoService, logger); err != nil {
			logger.Error("Failed to setup TLS certificates", "error", err)
			os.Exit(1)
		}

		// REST adapter
		restHandler := rest.NewHandler(
			logger,
			cfg,
			uiFiles,
			authService,
			messageService,
			domainService,
			queueService,
			routingService,
			statsService,
			resourceMonitorService,
			consumerGroupService,
			consumerGroupRepo,
			serviceRepo,
			accountRequestService,
		)
		restHandler.SetupRoutes(router)

		// WebSocket adapter
		wsHandler := websocket.NewHandler(messageService, ctx)
		router.HandleFunc(
			"/api/ws/domains/{domain}/queues/{queue}",
			func(w http.ResponseWriter, r *http.Request) {
				vars := mux.Vars(r)
				wsHandler.HandleConnection(w, r, vars["domain"], vars["queue"])
			},
		)

		router.Walk(func(route *mux.Route, router *mux.Router, ancestors []*mux.Route) error {
			pathTemplate, err := route.GetPathTemplate()
			if err != nil {
				logger.Error("ROUTE ERROR", "ERROR", err)
				return nil
			}
			methods, err := route.GetMethods()
			if err != nil {
				methods = []string{"ANY"}
			}
			logger.Info("ROUTE", "PATH", pathTemplate, "METHOD", methods)
			return nil
		})

		// start HTTP server
		httpAddr := fmt.Sprintf("%s:%d", cfg.HTTP.Address, cfg.HTTP.Port)
		server := &http.Server{
			Addr:         httpAddr,
			Handler:      router,
			ReadTimeout:  30 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  120 * time.Second,
		}

		// Configure TLS if enabled
		if cfg.HTTP.TLS {
			// Optional: Configure TLS settings for security
			server.TLSConfig = &tls.Config{
				MinVersion: tls.VersionTLS12, // Minimum TLS 1.2
				CipherSuites: []uint16{
					tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
					tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
					tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
				},
			}
		}

		// Start HTTP/HTTPS server
		go func() {
			if cfg.HTTP.TLS {
				logger.Info("HTTPS server listening",
					"URL", fmt.Sprintf("https://%s", httpAddr),
					"certFile", cfg.HTTP.CertFile,
					"keyFile", cfg.HTTP.KeyFile)

				if err := server.ListenAndServeTLS(cfg.HTTP.CertFile, cfg.HTTP.KeyFile); err != nil && err != http.ErrServerClosed {
					logger.Error("HTTPS server error", "error", err)
				}
			} else {
				logger.Info("HTTP server listening", "URL", fmt.Sprintf("http://%s", httpAddr))

				if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
					logger.Error("HTTP server error", "error", err)
				}
			}
		}()

		// stop HTTP server
		defer func() {
			// Important cleanup order: start with services that depend on others
			logger.Info("Cleaning up services...")

			// Suggested cleanup order (from most dependent to least dependent)
			if wsHandler != nil {
				wsHandler.Cleanup()
			}

			// if grpcServer != nil {
			// 	grpcServer.Stop()
			// }

			if messageService != nil {
				if cleanable, ok := messageService.(interface{ Cleanup() }); ok {
					cleanable.Cleanup()
				}
			}

			if queueService != nil {
				if cleanable, ok := queueService.(interface{ Cleanup() }); ok {
					cleanable.Cleanup()
				}
			}

			if statsService != nil {
				if cleanable, ok := statsService.(interface{ Cleanup() }); ok {
					cleanable.Cleanup()
				}
			}

			if routingService != nil {
				if cleanable, ok := routingService.(interface{ Cleanup() }); ok {
					cleanable.Cleanup()
				}
			}

			if domainService != nil {
				if cleanable, ok := domainService.(interface{ Cleanup() }); ok {
					cleanable.Cleanup()
				}
			}

			if slogAdapter, ok := logger.(*logging.SlogAdapter); ok {
				slogAdapter.Shutdown()
			}

			if fileWatcher != nil {
				if cleanable, ok := fileWatcher.(interface{ Cleanup() }); ok {
					cleanable.Cleanup()
				}
			}

			logger.Info("All services cleaned up")
		}()
	}

	// Middleware for debugging requests
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger.Info("Request", "METHOD", r.Method, "PATH", r.URL.Path)
			next.ServeHTTP(w, r)
		})
	})

	// Configure the gRPC adapter if enabled
	if cfg.GRPC.Enabled {
		grpcServer := grpc.NewServer(
			messageService,
			domainService,
			queueService,
			routingService,
			ctx,
		)
		grpcAddr := fmt.Sprintf("%s:%d", cfg.GRPC.Address, cfg.GRPC.Port)
		if err := grpcServer.Start(grpcAddr); err != nil {
			logger.Error("Failed to start gRPC server", "erroe", err)
		}

		// Stop the gRPC server at the end
		defer grpcServer.Stop()
	}

	// TODO: Implement adapters for AMQP and MQTT

	// Create predefined domains (if configured)
	for _, domainCfg := range cfg.Domains {
		logger.Info("Creating predefined domain", "domainName", domainCfg.Name)
		if err := createDomainFromConfig(ctx, domainService, queueService, routingService, domainCfg); err != nil {
			logger.Error("Failed to create domain",
				"domainName", domainCfg.Name,
				"ERROR", err)
		}
	}

	// Wait for signals for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Display the startup message
	logger.Info("GoRTMS started successfully")

	// Wait for shutdown signal
	sig := <-sigChan
	logger.Info("Received signal, shutting down gracefully...", "signal", sig)

	// Cancel the context to stop all goroutines
	cancel()

	logger.Info("Server shutdown complete")
}

func autoBootstrapAdmin(authService inbound.AuthService, logger outbound.Logger) error {
	users, err := authService.ListUsers()
	if err != nil {
		return fmt.Errorf("failed to check existing users: %w", err)
	}

	if len(users) > 0 {
		logger.Info("Users already exist, skipping auto-bootstrap")
		return nil
	}

	// Create default admin with standard credentials
	admin, err := authService.CreateUser("admin", "admin", model.RoleAdmin)
	if err != nil {
		return fmt.Errorf("failed to create default admin: %w", err)
	}

	logger.Info("🚀 Default admin created",
		"username", admin.Username,
		"password", "admin",
		"action", "Please change password after first login!")

	return nil
}

// createDomainFromConfig creates a domain from a configuration
func createDomainFromConfig(
	ctx context.Context,
	domainService inbound.DomainService,
	queueService inbound.QueueService,
	routingService inbound.RoutingService,
	config config.DomainConfig,
) error {
	// Create domain
	domainConfig := &model.DomainConfig{
		Name: config.Name,
		Schema: &model.Schema{
			Fields: make(map[string]model.FieldType),
		},
	}

	// If a schema is defined, convert the fields
	if schema, ok := config.Schema["fields"].(map[string]any); ok {
		for field, typeVal := range schema {
			if typeStr, ok := typeVal.(string); ok {
				domainConfig.Schema.Fields[field] = model.FieldType(typeStr)
			}
		}
	}

	if err := domainService.CreateDomain(ctx, domainConfig); err != nil {
		return fmt.Errorf("failed to create domain: %w", err)
	}

	// Create the queues
	for _, queueCfg := range config.Queues {
		queueConfig := queueCfg.Config

		// Default values for retry configuration
		if queueConfig.RetryEnabled && queueConfig.RetryConfig != nil {
			if queueConfig.RetryConfig.InitialDelay == 0 {
				queueConfig.RetryConfig.InitialDelay = 1 * time.Second
			}
			if queueConfig.RetryConfig.MaxDelay == 0 {
				queueConfig.RetryConfig.MaxDelay = 30 * time.Second
			}
			if queueConfig.RetryConfig.Factor <= 0 {
				queueConfig.RetryConfig.Factor = 2.0
			}
		}

		// Default values for circuit breaker
		if queueConfig.CircuitBreakerEnabled && queueConfig.CircuitBreakerConfig != nil {
			if queueConfig.CircuitBreakerConfig.ErrorThreshold <= 0 {
				queueConfig.CircuitBreakerConfig.ErrorThreshold = 0.5
			}
			if queueConfig.CircuitBreakerConfig.MinimumRequests <= 0 {
				queueConfig.CircuitBreakerConfig.MinimumRequests = 10
			}
			if queueConfig.CircuitBreakerConfig.OpenTimeout == 0 {
				queueConfig.CircuitBreakerConfig.OpenTimeout = 30 * time.Second
			}
			if queueConfig.CircuitBreakerConfig.SuccessThreshold <= 0 {
				queueConfig.CircuitBreakerConfig.SuccessThreshold = 5
			}
		}

		if err := queueService.CreateQueue(ctx, config.Name, queueCfg.Name, &queueConfig); err != nil {
			return fmt.Errorf("failed to create queue %s: %w", queueCfg.Name, err)
		}
	}

	// Add routing rules
	for _, routeCfg := range config.Routes {
		// Create a rule with a simple JSON predicate
		rulePredicate := model.JSONPredicate{
			Type:  routeCfg.Predicate["type"].(string),
			Field: routeCfg.Predicate["field"].(string),
			Value: routeCfg.Predicate["value"],
		}

		rule := &model.RoutingRule{
			SourceQueue:      routeCfg.SourceQueue,
			DestinationQueue: routeCfg.DestinationQueue,
			Predicate:        rulePredicate,
		}

		if err := routingService.AddRoutingRule(ctx, config.Name, rule); err != nil {
			return fmt.Errorf("failed to add routing rule: %w", err)
		}
	}

	return nil
}
