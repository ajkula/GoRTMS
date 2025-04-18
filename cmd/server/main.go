package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	// Imports des packages GoRTMS
	"github.com/ajkula/GoRTMS/adapter/inbound/grpc"
	"github.com/ajkula/GoRTMS/adapter/inbound/rest"
	"github.com/ajkula/GoRTMS/adapter/inbound/websocket"
	"github.com/ajkula/GoRTMS/adapter/outbound/storage/memory"
	"github.com/ajkula/GoRTMS/config"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/service"

	// Import temporaires pour la compilation
	"github.com/ajkula/GoRTMS/domain/port/inbound"
)

func main() {
	// Traiter les arguments de ligne de commande
	var configPath string
	var generateConfig bool
	var showVersion bool

	flag.StringVar(&configPath, "config", "config.yaml", "Path to configuration file")
	flag.BoolVar(&generateConfig, "generate-config", false, "Generate default configuration file")
	flag.BoolVar(&showVersion, "version", false, "Show version information")
	flag.Parse()

	// Afficher les informations de version
	if showVersion {
		fmt.Println("GoRTMS Version 1.0.0")
		os.Exit(0)
	}

	// Générer un fichier de configuration par défaut
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

	// Charger la configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		fmt.Printf("Error loading config: %v\n", err)
		os.Exit(1)
	}

	// Configurer la journalisation
	// TODO: Configurer un logger plus avancé

	log.Println("Starting GoRTMS...")
	log.Printf("Node ID: %s", cfg.General.NodeID)
	log.Printf("Data directory: %s", cfg.General.DataDir)

	// Créer le répertoire de données s'il n'existe pas
	if err := os.MkdirAll(cfg.General.DataDir, 0755); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	// Créer un contexte annulable
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Initialiser les repositories (adaptateurs sortants)
	messageRepo := memory.NewMessageRepository()
	domainRepo := memory.NewDomainRepository()
	subscriptionReg := memory.NewSubscriptionRegistry()

	// Créer les services (implémentations du domaine)
	statsService := service.NewStatsService(domainRepo, messageRepo, ctx)
	queueService := service.NewQueueService(domainRepo, statsService, ctx)
	messageService := service.NewMessageService(
		domainRepo,
		messageRepo,
		subscriptionReg,
		queueService,
		statsService,
	)
	domainService := service.NewDomainService(domainRepo)
	routingService := service.NewRoutingService(domainRepo)

	// Créer le routeur HTTP
	router := mux.NewRouter()

	// Configurer les adaptateurs entrants
	if cfg.HTTP.Enabled {
		// Adaptateur REST
		restHandler := rest.NewHandler(
			messageService,
			domainService,
			queueService,
			routingService,
			statsService,
		)
		restHandler.SetupRoutes(router)

		// Adaptateur WebSocket
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
				log.Println("ROUTE ERROR:", err)
				return nil
			}
			methods, err := route.GetMethods()
			if err != nil {
				methods = []string{"ANY"}
			}
			log.Printf("ROUTE: %s [%v]", pathTemplate, methods)
			return nil
		})

		// Démarrer le serveur HTTP
		httpAddr := fmt.Sprintf("%s:%d", cfg.HTTP.Address, cfg.HTTP.Port)
		server := &http.Server{
			Addr:    httpAddr,
			Handler: router,
		}

		go func() {
			log.Printf("HTTP server listening on %s", httpAddr)
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("HTTP server error: %v", err)
			}
		}()

		// Arrêter le serveur HTTP à la fin
		defer func() {
			shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
			defer shutdownCancel()
			if err := server.Shutdown(shutdownCtx); err != nil {
				log.Printf("HTTP server shutdown error: %v", err)
			}
			// Nettoyer les ressources des services
			if cleanable, ok := queueService.(interface{ Cleanup() }); ok {
				cleanable.Cleanup()
			}
			if cleanable, ok := messageService.(interface{ Cleanup() }); ok {
				cleanable.Cleanup()
			}
		}()
	}

	// Middleware pour déboguer les requêtes
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			log.Printf("Request: %s %s", r.Method, r.URL.Path)
			next.ServeHTTP(w, r)
		})
	})

	// Configurer l'adaptateur gRPC si activé
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
			log.Fatalf("Failed to start gRPC server: %v", err)
		}

		// Arrêter le serveur gRPC à la fin
		defer grpcServer.Stop()
	}

	// TODO: Implémenter les adaptateurs pour AMQP et MQTT

	// Créer les domaines prédéfinis (si configurés)
	for _, domainCfg := range cfg.Domains {
		log.Printf("Creating predefined domain: %s", domainCfg.Name)
		if err := createDomainFromConfig(ctx, domainService, queueService, routingService, domainCfg); err != nil {
			log.Printf("Failed to create domain %s: %v", domainCfg.Name, err)
		}
	}

	// Attendre les signaux pour un arrêt gracieux
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Afficher le message de démarrage
	log.Println("GoRTMS started successfully")

	// Attendre le signal d'arrêt
	sig := <-sigChan
	log.Printf("Received signal %v, shutting down gracefully...", sig)

	// Annuler le contexte pour arrêter toutes les goroutines
	cancel()

	log.Println("Server shutdown complete")
}

// createDomainFromConfig crée un domaine à partir d'une configuration
func createDomainFromConfig(
	ctx context.Context,
	domainService inbound.DomainService,
	queueService inbound.QueueService,
	routingService inbound.RoutingService,
	config config.DomainConfig,
) error {
	// Créer le domaine
	domainConfig := &model.DomainConfig{
		Name: config.Name,
		Schema: &model.Schema{
			Fields: make(map[string]model.FieldType),
		},
	}

	// Si un schéma est défini, convertir les champs
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

	// Créer les files d'attente
	for _, queueCfg := range config.Queues {
		queueConfig := queueCfg.Config

		// Valeurs par défaut pour la config de retry
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

		// Valeurs par défaut pour circuit breaker
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

	// Ajouter les règles de routage
	for _, routeCfg := range config.Routes {
		// Créer une règle avec un prédicat JSON simple
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
