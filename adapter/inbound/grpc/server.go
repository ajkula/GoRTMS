package grpc

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	proto "github.com/ajkula/GoRTMS/adapter/inbound/grpc/proto/generated"
	"github.com/ajkula/GoRTMS/domain/model"
	"github.com/ajkula/GoRTMS/domain/port/inbound"
)

// Server implémente le service gRPC GoRTMS
type Server struct {
	proto.UnimplementedGoRTMSServer
	messageService inbound.MessageService
	domainService  inbound.DomainService
	queueService   inbound.QueueService
	routingService inbound.RoutingService
	grpcServer     *grpc.Server
	rootCtx        context.Context
}

// NewServer crée un nouveau serveur gRPC
func NewServer(
	messageService inbound.MessageService,
	domainService inbound.DomainService,
	queueService inbound.QueueService,
	routingService inbound.RoutingService,
	rootCtx context.Context,
) *Server {
	return &Server{
		messageService: messageService,
		domainService:  domainService,
		queueService:   queueService,
		routingService: routingService,
		rootCtx:        rootCtx,
	}
}

// Start démarre le serveur gRPC
func (s *Server) Start(address string) error {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to listen: %v", err)
	}

	s.grpcServer = grpc.NewServer()
	proto.RegisterGoRTMSServer(s.grpcServer, s)

	go func() {
		if err := s.grpcServer.Serve(lis); err != nil {
			fmt.Printf("failed to serve: %v\n", err)
		}
	}()

	fmt.Printf("gRPC server started on %s\n", address)
	return nil
}

// Stop arrête le serveur gRPC
func (s *Server) Stop() {
	log.Println("Stopping gRPC server...")

	if s.grpcServer != nil {
		// Utiliser un timeout pour GracefulStop
		stopped := make(chan struct{})
		go func() {
			s.grpcServer.GracefulStop()
			close(stopped)
		}()

		// Attendre avec timeout
		select {
		case <-stopped:
			log.Println("gRPC server stopped gracefully")
		case <-time.After(10 * time.Second):
			log.Println("gRPC server stop timed out, forcing shutdown")
			s.grpcServer.Stop()
		}
	}

	log.Println("gRPC server shutdown complete")
}

// ListDomains liste tous les domaines
func (s *Server) ListDomains(
	ctx context.Context,
	req *proto.ListDomainsRequest,
) (*proto.ListDomainsResponse, error) {
	domains, err := s.domainService.ListDomains(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to list domains: %v", err)
	}

	response := &proto.ListDomainsResponse{
		Domains: make([]*proto.DomainInfo, len(domains)),
	}

	for i, domain := range domains {
		response.Domains[i] = &proto.DomainInfo{
			Name: domain.Name,
		}
	}

	return response, nil
}

// CreateDomain crée un nouveau domaine
func (s *Server) CreateDomain(
	ctx context.Context,
	req *proto.CreateDomainRequest,
) (*proto.CreateDomainResponse, error) {
	// Convertir le schéma
	schema := &model.Schema{
		Fields: make(map[string]model.FieldType),
	}

	for field, typeStr := range req.Schema.Fields {
		schema.Fields[field] = model.FieldType(typeStr)
	}

	// Convertir les configurations de files d'attente
	queueConfigs := make(map[string]model.QueueConfig)
	for name, config := range req.QueueConfigs {
		queueConfigs[name] = model.QueueConfig{
			IsPersistent: config.IsPersistent,
			MaxSize:      int(config.MaxSize),
			TTL:          time.Duration(config.TtlMs) * time.Millisecond,
		}
	}

	// Convertir les règles de routage
	routingRules := make([]*model.RoutingRule, len(req.RoutingRules))
	for i, rule := range req.RoutingRules {
		routingRules[i] = &model.RoutingRule{
			SourceQueue:      rule.SourceQueue,
			DestinationQueue: rule.DestinationQueue,
			Predicate: model.JSONPredicate{
				Type:  rule.Predicate.Type,
				Field: rule.Predicate.Field,
				Value: rule.Predicate.Value,
			},
		}
	}

	// Créer la configuration du domaine
	config := &model.DomainConfig{
		Name:         req.Name,
		Schema:       schema,
		QueueConfigs: queueConfigs,
		RoutingRules: routingRules,
	}

	// Créer le domaine
	if err := s.domainService.CreateDomain(ctx, config); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create domain: %v", err)
	}

	return &proto.CreateDomainResponse{
		DomainId: req.Name,
	}, nil
}

// GetDomain récupère les détails d'un domaine
func (s *Server) GetDomain(
	ctx context.Context,
	req *proto.GetDomainRequest,
) (*proto.DomainResponse, error) {
	domain, err := s.domainService.GetDomain(ctx, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Domain not found: %v", err)
	}

	// Convertir le schéma
	schemaInfo := &proto.SchemaInfo{
		Fields: make(map[string]string),
	}

	if domain.Schema != nil {
		for field, fieldType := range domain.Schema.Fields {
			schemaInfo.Fields[field] = string(fieldType)
		}
	}

	// Convertir les files d'attente
	queues := make([]*proto.QueueInfo, 0, len(domain.Queues))
	for _, queue := range domain.Queues {
		queues = append(queues, &proto.QueueInfo{
			Name:         queue.Name,
			MessageCount: int32(queue.MessageCount),
		})
	}

	// Convertir les règles de routage
	rules := make([]*proto.RoutingRuleInfo, 0)
	for sourceQueue, destRules := range domain.Routes {
		for destQueue, rule := range destRules {
			// Extraire le prédicat
			var predicate *proto.Predicate

			switch p := rule.Predicate.(type) {
			case model.JSONPredicate:
				predicate = &proto.Predicate{
					Type:  p.Type,
					Field: p.Field,
					Value: fmt.Sprintf("%v", p.Value),
				}
			}

			rules = append(rules, &proto.RoutingRuleInfo{
				SourceQueue:      sourceQueue,
				DestinationQueue: destQueue,
				Predicate:        predicate,
			})
		}
	}

	return &proto.DomainResponse{
		Name:         domain.Name,
		Schema:       schemaInfo,
		Queues:       queues,
		RoutingRules: rules,
	}, nil
}

// DeleteDomain supprime un domaine
func (s *Server) DeleteDomain(
	ctx context.Context,
	req *proto.DeleteDomainRequest,
) (*proto.StatusResponse, error) {
	if err := s.domainService.DeleteDomain(ctx, req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to delete domain: %v", err)
	}

	return &proto.StatusResponse{
		Success: true,
		Message: "Domain deleted successfully",
	}, nil
}

// ListQueues liste toutes les files d'attente d'un domaine
func (s *Server) ListQueues(
	ctx context.Context,
	req *proto.ListQueuesRequest,
) (*proto.ListQueuesResponse, error) {
	queues, err := s.queueService.ListQueues(ctx, req.DomainName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to list queues: %v", err)
	}

	response := &proto.ListQueuesResponse{
		Queues: make([]*proto.QueueInfo, len(queues)),
	}

	for i, queue := range queues {
		response.Queues[i] = &proto.QueueInfo{
			Name:         queue.Name,
			MessageCount: int32(queue.MessageCount),
		}
	}

	return response, nil
}

// CreateQueue crée une nouvelle file d'attente
func (s *Server) CreateQueue(
	ctx context.Context,
	req *proto.CreateQueueRequest,
) (*proto.CreateQueueResponse, error) {
	// Convertir la configuration
	config := &model.QueueConfig{
		IsPersistent: req.Config.IsPersistent,
		MaxSize:      int(req.Config.MaxSize),
		TTL:          time.Duration(req.Config.TtlMs) * time.Millisecond,
	}

	// Créer la file d'attente
	if err := s.queueService.CreateQueue(ctx, req.DomainName, req.Name, config); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to create queue: %v", err)
	}

	return &proto.CreateQueueResponse{
		QueueId: req.Name,
	}, nil
}

// GetQueue récupère les détails d'une file d'attente
func (s *Server) GetQueue(
	ctx context.Context,
	req *proto.GetQueueRequest,
) (*proto.QueueResponse, error) {
	queue, err := s.queueService.GetQueue(ctx, req.DomainName, req.Name)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "Queue not found: %v", err)
	}

	// Convertir la configuration
	config := &proto.QueueConfig{
		IsPersistent: queue.Config.IsPersistent,
		MaxSize:      int32(queue.Config.MaxSize),
		TtlMs:        int64(queue.Config.TTL / time.Millisecond),
	}

	return &proto.QueueResponse{
		Name:         queue.Name,
		MessageCount: int32(queue.MessageCount),
		Config:       config,
	}, nil
}

// DeleteQueue supprime une file d'attente
func (s *Server) DeleteQueue(
	ctx context.Context,
	req *proto.DeleteQueueRequest,
) (*proto.StatusResponse, error) {
	if err := s.queueService.DeleteQueue(ctx, req.DomainName, req.Name); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to delete queue: %v", err)
	}

	return &proto.StatusResponse{
		Success: true,
		Message: "Queue deleted successfully",
	}, nil
}

// PublishMessage publie un message dans une file d'attente
func (s *Server) PublishMessage(
	ctx context.Context,
	req *proto.PublishMessageRequest,
) (*proto.PublishMessageResponse, error) {
	// Convertir le message
	message := &model.Message{
		ID:        req.Message.Id,
		Payload:   req.Message.Payload,
		Headers:   req.Message.Headers,
		Timestamp: time.Unix(0, req.Message.Timestamp),
	}

	// Convertir les métadonnées
	if req.Message.Metadata != nil {
		message.Metadata = make(map[string]any)
		for key, value := range req.Message.Metadata {
			message.Metadata[key] = value
		}
	}

	// Publier le message
	if err := s.messageService.PublishMessage(req.DomainName, req.QueueName, message); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to publish message: %v", err)
	}

	return &proto.PublishMessageResponse{
		MessageId: message.ID,
	}, nil
}

// ConsumeMessages consomme des messages d'une file d'attente
func (s *Server) ConsumeMessages(
	ctx context.Context,
	req *proto.ConsumeMessagesRequest,
) (*proto.ConsumeMessagesResponse, error) {
	// Créer un contexte avec timeout si nécessaire
	if req.TimeoutSeconds > 0 {
		var cancel context.CancelFunc
		_, cancel = context.WithTimeout(ctx, time.Duration(req.TimeoutSeconds)*time.Second)
		defer cancel()
	}

	// Récupérer les messages
	var messages []*model.Message
	for i := 0; i < int(req.MaxMessages); i++ {
		message, err := s.messageService.ConsumeMessageWithGroup(ctx, req.DomainName, req.QueueName, "", &inbound.ConsumeOptions{})
		if err != nil {
			return nil, status.Errorf(codes.Internal, "Failed to consume message: %v", err)
		}

		if message == nil {
			break
		}

		messages = append(messages, message)
	}

	// Convertir les messages
	protoMessages := make([]*proto.Message, len(messages))
	for i, message := range messages {
		// Convertir les métadonnées
		metadata := make(map[string]string)
		for key, value := range message.Metadata {
			metadata[key] = fmt.Sprintf("%v", value)
		}

		protoMessages[i] = &proto.Message{
			Id:        message.ID,
			Payload:   message.Payload,
			Headers:   message.Headers,
			Metadata:  metadata,
			Timestamp: message.Timestamp.UnixNano(),
		}
	}

	return &proto.ConsumeMessagesResponse{
		Messages: protoMessages,
	}, nil
}

// SubscribeToQueue s'abonne à une file d'attente
func (s *Server) SubscribeToQueue(
	req *proto.SubscribeRequest,
	stream proto.GoRTMS_SubscribeToQueueServer,
) error {
	// Créer un canal pour recevoir les messages
	messageChan := make(chan *model.Message)

	// Créer un handler qui enverra les messages au canal
	handler := func(message *model.Message) error {
		select {
		case messageChan <- message:
			return nil
		case <-stream.Context().Done():
			return stream.Context().Err()
		}
	}

	// S'abonner à la file d'attente
	subscriptionID, err := s.messageService.SubscribeToQueue(
		req.DomainName,
		req.QueueName,
		handler,
	)

	if err != nil {
		return status.Errorf(codes.Internal, "Failed to subscribe: %v", err)
	}

	// Se désinscrire à la fin
	defer s.messageService.UnsubscribeFromQueue(
		req.DomainName,
		req.QueueName,
		subscriptionID,
	)

	// Boucle pour envoyer les messages au client
	for {
		select {
		case <-stream.Context().Done():
			return stream.Context().Err()
		case message := <-messageChan:
			// Convertir les métadonnées
			metadata := make(map[string]string)
			for key, value := range message.Metadata {
				metadata[key] = fmt.Sprintf("%v", value)
			}

			// Convertir le message
			protoMessage := &proto.Message{
				Id:        message.ID,
				Payload:   message.Payload,
				Headers:   message.Headers,
				Metadata:  metadata,
				Timestamp: message.Timestamp.UnixNano(),
			}

			// Envoyer le message au client
			if err := stream.Send(&proto.MessageResponse{
				Message: protoMessage,
			}); err != nil {
				return status.Errorf(codes.Internal, "Failed to send message: %v", err)
			}
		}
	}
}

// AddRoutingRule ajoute une règle de routage
func (s *Server) AddRoutingRule(
	ctx context.Context,
	req *proto.AddRoutingRuleRequest,
) (*proto.StatusResponse, error) {
	// Convertir le prédicat
	var predicate any
	if req.Rule.Predicate != nil {
		// Pour simplifier, on utilise un JSONPredicate
		predicate = model.JSONPredicate{
			Type:  req.Rule.Predicate.Type,
			Field: req.Rule.Predicate.Field,
			Value: req.Rule.Predicate.Value,
		}
	}

	// Créer la règle
	rule := &model.RoutingRule{
		SourceQueue:      req.Rule.SourceQueue,
		DestinationQueue: req.Rule.DestinationQueue,
		Predicate:        predicate,
	}

	// Ajouter la règle
	if err := s.routingService.AddRoutingRule(ctx, req.DomainName, rule); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to add routing rule: %v", err)
	}

	return &proto.StatusResponse{
		Success: true,
		Message: "Routing rule added successfully",
	}, nil
}

// RemoveRoutingRule supprime une règle de routage
func (s *Server) RemoveRoutingRule(
	ctx context.Context,
	req *proto.RemoveRoutingRuleRequest,
) (*proto.StatusResponse, error) {
	if err := s.routingService.RemoveRoutingRule(
		ctx,
		req.DomainName,
		req.SourceQueue,
		req.DestinationQueue,
	); err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to remove routing rule: %v", err)
	}

	return &proto.StatusResponse{
		Success: true,
		Message: "Routing rule removed successfully",
	}, nil
}

// ListRoutingRules liste toutes les règles de routage d'un domaine
func (s *Server) ListRoutingRules(
	ctx context.Context,
	req *proto.ListRoutingRulesRequest,
) (*proto.ListRoutingRulesResponse, error) {
	rules, err := s.routingService.ListRoutingRules(ctx, req.DomainName)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "Failed to list routing rules: %v", err)
	}

	protoRules := make([]*proto.RoutingRuleInfo, len(rules))
	for i, rule := range rules {
		// Convertir le prédicat
		var predicate *proto.Predicate

		switch p := rule.Predicate.(type) {
		case model.JSONPredicate:
			predicate = &proto.Predicate{
				Type:  p.Type,
				Field: p.Field,
				Value: fmt.Sprintf("%v", p.Value),
			}
		}

		protoRules[i] = &proto.RoutingRuleInfo{
			SourceQueue:      rule.SourceQueue,
			DestinationQueue: rule.DestinationQueue,
			Predicate:        predicate,
		}
	}

	return &proto.ListRoutingRulesResponse{
		Rules: protoRules,
	}, nil
}
