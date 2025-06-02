package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "net/http/pprof"

	"github.com/gorilla/mux"

	"github.com/ajkula/GoRTMS/adapter/inbound/grpc"
	"github.com/ajkula/GoRTMS/adapter/inbound/rest"
	"github.com/ajkula/GoRTMS/adapter/inbound/websocket"
	"github.com/ajkula/GoRTMS/adapter/outbound/logging"
	"github.com/ajkula/GoRTMS/adapter/outbound/storage/memory"
	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/service"

	// Temporary imports for compilation
	"github.com/ajkula/GoRTMS/domain/port/inbound"
)

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

	// Create HTTP router
	router := mux.NewRouter()

	// Configure the incoming adapters
	if cfg.HTTP.Enabled {
		// REST adapter
		restHandler := rest.NewHandler(
			logger,
			messageService,
			domainService,
			queueService,
			routingService,
			statsService,
			resourceMonitorService,
			consumerGroupService,
			consumerGroupRepo,
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
			ReadTimeout:  5 * time.Second,
			WriteTimeout: 10 * time.Second,
		}

		go func() {
			logger.Info("HTTP server listening", "URL", httpAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				_, shutdownCancel := context.WithTimeout(ctx, 1*time.Second)
				defer shutdownCancel()
				logger.Error("HTTP server", "ERROR", err)
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
