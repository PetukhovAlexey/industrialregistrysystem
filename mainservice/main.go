package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"industrialregistrysystem/base/api"
	"industrialregistrysystem/mainservice/cache"
)

// DatabaseConnection представляет аутентифицированное подключение к базе данных
type DatabaseConnection struct {
	ServiceID    string
	DNSName      string
	ConnectedAt  time.Time
	PeerInfo     *peer.Peer
	CommandChan  chan *api.CommandRequest  // Канал для отправки команд этой БД
	ResponseChan chan *api.CommandResponse // Канал для получения ответов от этой БД
}

// DatabaseRegistry реестр аутентифицированных подключений к БД
type DatabaseRegistry struct {
	mu          sync.RWMutex
	connections map[string]*DatabaseConnection
}

func NewDatabaseRegistry() *DatabaseRegistry {
	return &DatabaseRegistry{
		connections: make(map[string]*DatabaseConnection),
	}
}

// RegisterDatabase регистрирует новое аутентифицированное подключение БД
func (registry *DatabaseRegistry) RegisterDatabase(ctx context.Context, serviceID string) (*DatabaseConnection, error) {
	peerInfo, ok := peer.FromContext(ctx)
	if !ok {
		return nil, fmt.Errorf("no peer information in context")
	}

	// Проверяем TLS аутентификацию
	tlsAuth, ok := peerInfo.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, fmt.Errorf("connection is not using TLS")
	}

	if len(tlsAuth.State.PeerCertificates) == 0 {
		return nil, fmt.Errorf("no client certificate provided")
	}

	clientCertificate := tlsAuth.State.PeerCertificates[0]

	// Проверяем, что сертификат выдан для роли database
	dnsName, valid := registry.validateDatabaseCertificate(clientCertificate)
	if !valid {
		return nil, fmt.Errorf("invalid database certificate: DNSNames=%v, OU=%v",
			clientCertificate.DNSNames, clientCertificate.Subject.OrganizationalUnit)
	}

	connection := &DatabaseConnection{
		ServiceID:    serviceID,
		DNSName:      dnsName,
		ConnectedAt:  time.Now(),
		PeerInfo:     peerInfo,
		CommandChan:  make(chan *api.CommandRequest, 100),
		ResponseChan: make(chan *api.CommandResponse, 100),
	}

	registry.mu.Lock()
	registry.connections[serviceID] = connection
	registry.mu.Unlock()

	log.Printf("✅ Database registered: %s (DNS: %s, Addr: %s)",
		serviceID, connection.DNSName, peerInfo.Addr.String())

	return connection, nil
}

// RegisterDatabaseFromStream регистрирует базу данных из CommandStream
func (registry *DatabaseRegistry) RegisterDatabaseFromStream(stream context.Context, serviceID string) (*DatabaseConnection, error) {
	peerInfo, ok := peer.FromContext(stream)
	if !ok {
		return nil, fmt.Errorf("no peer information in stream context")
	}

	// Проверяем TLS аутентификацию
	tlsAuth, ok := peerInfo.AuthInfo.(credentials.TLSInfo)
	if !ok {
		return nil, fmt.Errorf("connection is not using TLS")
	}

	if len(tlsAuth.State.PeerCertificates) == 0 {
		return nil, fmt.Errorf("no client certificate provided")
	}

	clientCertificate := tlsAuth.State.PeerCertificates[0]

	// Проверяем, что сертификат выдан для роли database
	dnsName, valid := registry.validateDatabaseCertificate(clientCertificate)
	if !valid {
		return nil, fmt.Errorf("invalid database certificate: DNSNames=%v, OU=%v",
			clientCertificate.DNSNames, clientCertificate.Subject.OrganizationalUnit)
	}

	connection := &DatabaseConnection{
		ServiceID:    serviceID,
		DNSName:      dnsName,
		ConnectedAt:  time.Now(),
		PeerInfo:     peerInfo,
		CommandChan:  make(chan *api.CommandRequest, 100),
		ResponseChan: make(chan *api.CommandResponse, 100),
	}

	registry.mu.Lock()
	registry.connections[serviceID] = connection
	registry.mu.Unlock()

	log.Printf("✅ Database registered from stream: %s (DNS: %s, Addr: %s)",
		serviceID, connection.DNSName, peerInfo.Addr.String())

	return connection, nil
}

// validateDatabaseCertificate проверяет валидность сертификата БД
func (registry *DatabaseRegistry) validateDatabaseCertificate(certificate *x509.Certificate) (string, bool) {
	// Проверяем DNS Names
	if len(certificate.DNSNames) == 0 {
		return "", false
	}

	// Ищем DNS Name, соответствующее паттерну базы данных
	var databaseDNS string
	for _, dnsName := range certificate.DNSNames {
		if dnsName == "database" || dnsName == "database.industrialregistrysystem" {
			databaseDNS = dnsName
			break
		}
	}

	if databaseDNS == "" {
		return "", false
	}

	// Проверяем Organizational Unit
	hasDatabaseOU := false
	for _, organizationalUnit := range certificate.Subject.OrganizationalUnit {
		if organizationalUnit == "Database" {
			hasDatabaseOU = true
			break
		}
	}

	if !hasDatabaseOU {
		return "", false
	}

	// Проверяем Organization
	hasCorrectOrganization := false
	for _, organization := range certificate.Subject.Organization {
		if organization == "IndustrialRegistrySystem" {
			hasCorrectOrganization = true
			break
		}
	}

	return databaseDNS, hasCorrectOrganization
}

// GetDatabase возвращает подключение к БД по ID
func (registry *DatabaseRegistry) GetDatabase(serviceID string) (*DatabaseConnection, bool) {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	connection, exists := registry.connections[serviceID]
	return connection, exists
}

// ListDatabases возвращает список всех зарегистрированных БД
func (registry *DatabaseRegistry) ListDatabases() []*DatabaseConnection {
	registry.mu.RLock()
	defer registry.mu.RUnlock()

	connections := make([]*DatabaseConnection, 0, len(registry.connections))
	for _, connection := range registry.connections {
		connections = append(connections, connection)
	}

	return connections
}

// RemoveDatabase удаляет подключение БД из реестра
func (registry *DatabaseRegistry) RemoveDatabase(serviceID string) {
	registry.mu.Lock()
	defer registry.mu.Unlock()

	if connection, exists := registry.connections[serviceID]; exists {
		log.Printf("🗑️ Database unregistered: %s (DNS: %s)", serviceID, connection.DNSName)
		close(connection.CommandChan)
		close(connection.ResponseChan)
		delete(registry.connections, serviceID)
	}
}

// SendCommandToDatabase отправляет команду конкретной базе данных
func (registry *DatabaseRegistry) SendCommandToDatabase(serviceID string, command *api.CommandRequest) error {
	registry.mu.RLock()
	connection, exists := registry.connections[serviceID]
	registry.mu.RUnlock()

	if !exists {
		return fmt.Errorf("database with ID %s not found", serviceID)
	}

	select {
	case connection.CommandChan <- command:
		log.Printf("📤 Command sent to database %s: %s", serviceID, command.RequestId)
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("timeout sending command to database %s", serviceID)
	}
}

// UserDataService реализует оба сервиса: DataService и DatabaseService
type UserDataService struct {
	api.UnimplementedDataServiceServer
	api.UnimplementedDatabaseServiceServer
	cache            cache.Cache
	databaseRegistry *DatabaseRegistry
	pendingRequests  sync.Map // map[string]chan *api.CommandResponse - ожидающие ответы
}

func NewUserDataService() *UserDataService {
	// Используем фабрику для создания кэша с метриками
	cacheWithMetrics := cache.NewFIFO3CacheWithMetrics(1000)
	
	return &UserDataService{
		cache:            cacheWithMetrics,
		databaseRegistry: NewDatabaseRegistry(),
	}
}

// RegisterDatabase регистрирует подключение базы данных с аутентификацией
func (service *UserDataService) RegisterDatabase(ctx context.Context, request *api.DatabaseRegistrationRequest) (*api.DatabaseRegistrationResponse, error) {
	connection, err := service.databaseRegistry.RegisterDatabase(ctx, request.ServiceId)
	if err != nil {
		log.Printf("❌ Database registration failed: %v", err)
		return &api.DatabaseRegistrationResponse{
			Success:      false,
			ErrorMessage: err.Error(),
		}, nil
	}

	log.Printf("✅ Database %s ready to receive commands", request.ServiceId)

	return &api.DatabaseRegistrationResponse{
		Success:    true,
		ServiceId:  connection.ServiceID,
		CommonName: connection.DNSName,
		Timestamp:  connection.ConnectedAt.Format(time.RFC3339),
	}, nil
}

// CommandStream обрабатывает поток команд - СЕРВЕР ОТПРАВЛЯЕТ CommandRequest, КЛИЕНТЫ ОТПРАВЛЯЮТ CommandResponse
func (service *UserDataService) CommandStream(stream api.DatabaseService_CommandStreamServer) error {
	// Получаем информацию о подключении
	peerInfo, ok := peer.FromContext(stream.Context())
	if !ok {
		return fmt.Errorf("no peer information in context")
	}

	log.Printf("🔗 New command stream connection from: %s", peerInfo.Addr.String())

	// Получаем ServiceID из TLS сертификата
	serviceID := service.getServiceIDFromContext(stream.Context())
	if serviceID == "" {
		return fmt.Errorf("cannot determine service ID from certificate")
	}

	// Проверяем, зарегистрирована ли уже база данных
	_, exists := service.databaseRegistry.GetDatabase(serviceID)
	if !exists {
		// Если не зарегистрирована, регистрируем её автоматически из потока
		log.Printf("📝 Auto-registering database from stream: %s", serviceID)
		_, err := service.databaseRegistry.RegisterDatabaseFromStream(stream.Context(), serviceID)
		if err != nil {
			log.Printf("❌ Failed to auto-register database %s: %v", serviceID, err)
			return err
		}
	}

	// Получаем соединение с БД
	connection, exists := service.databaseRegistry.GetDatabase(serviceID)
	if !exists {
		return fmt.Errorf("database %s not found after registration", serviceID)
	}

	log.Printf("🔧 Database %s connected to command stream", serviceID)

	// Горутина для отправки команд клиенту
	go func() {
		for command := range connection.CommandChan {
			log.Printf("📤 Sending command to database %s: %s", serviceID, command.RequestId)
			if err := stream.Send(command); err != nil {
				log.Printf("❌ Failed to send command to database %s: %v", serviceID, err)
				return
			}
		}
	}()

	// Основной цикл обработки ответов от клиента
	for {
		// Получаем CommandResponse от клиента (базы данных)
		commandResponse, receiveError := stream.Recv()
		if receiveError != nil {
			log.Printf("❌ Error receiving from database %s: %v", serviceID, receiveError)
			
			// Удаляем базу данных из реестра при отключении
			service.databaseRegistry.RemoveDatabase(serviceID)
			return receiveError
		}

		log.Printf("📨 Received response from database %s for request: %s", serviceID, commandResponse.RequestId)

		// Обрабатываем ответ от базы данных
		service.processDatabaseResponse(commandResponse)
	}
}

// getServiceIDFromContext извлекает ServiceID из TLS контекста
func (service *UserDataService) getServiceIDFromContext(ctx context.Context) string {
	peerInfo, ok := peer.FromContext(ctx)
	if !ok {
		return ""
	}

	tlsAuth, ok := peerInfo.AuthInfo.(credentials.TLSInfo)
	if !ok || len(tlsAuth.State.PeerCertificates) == 0 {
		return ""
	}

	// Используем первый DNS Name как ServiceID
	// В сертификате database DNSNames должно содержать "database"
	cert := tlsAuth.State.PeerCertificates[0]
	if len(cert.DNSNames) > 0 {
		return cert.DNSNames[0]
	}
	
	return ""
}

// processDatabaseResponse обрабатывает ответ от базы данных
func (service *UserDataService) processDatabaseResponse(response *api.CommandResponse) {
	// Здесь можно добавить логику обработки ответов от базы данных
	// Например, кэширование результатов, уведомление ожидающих горутин и т.д.
	log.Printf("🔧 Processing response for request: %s", response.RequestId)
	
	// Кэшируем успешные ответы
	if response.Response != nil {
		cacheKey := service.generateCacheKey(response.RequestId, response.Response)
		if cacheKey != "" {
			service.cache.Set(cacheKey, response)
			log.Printf("💾 Response cached with key: %s", cacheKey)
		}
	}
	
	// Простая обработка: логируем тип ответа
	switch response.Response.(type) {
	case *api.CommandResponse_Organization:
		log.Printf("📊 Received organization data for request: %s", response.RequestId)
	case *api.CommandResponse_User:
		log.Printf("👤 Received user data for request: %s", response.RequestId)
	case *api.CommandResponse_Entity:
		log.Printf("📄 Received entity data for request: %s", response.RequestId)
	case *api.CommandResponse_List:
		log.Printf("📋 Received list data for request: %s", response.RequestId)
	case *api.CommandResponse_Error:
		errorResp := response.GetError()
		log.Printf("❌ Received error for request %s: %s", response.RequestId, errorResp.Message)
	case *api.CommandResponse_Ready:
		readyResp := response.GetReady()
		log.Printf("✅ Received ready message for request %s: %s", response.RequestId, readyResp.ServiceName)
	default:
		log.Printf("📨 Received response type for request %s", response.RequestId)
	}
}

// generateCacheKey генерирует ключ кэша на основе запроса и ответа
func (service *UserDataService) generateCacheKey(requestId string, response interface{}) string {
	// Генерируем ключ кэша на основе типа данных и идентификатора
	switch resp := response.(type) {
	case *api.CommandResponse_Organization:
		if org := resp.Organization; org != nil && org.Organization != nil {
			return fmt.Sprintf("org:%d", org.Organization.Id)
		}
	case *api.CommandResponse_User:
		if user := resp.User; user != nil && user.User != nil {
			return fmt.Sprintf("user:%d", user.User.Id)
		}
	case *api.CommandResponse_Entity:
		if entityResp := resp.Entity; entityResp != nil && entityResp.Entity != nil {
			// Для Entity используем комбинацию table_name и полей
			if id, ok := entityResp.Entity.Fields["id"]; ok {
				return fmt.Sprintf("entity:%s:%s", entityResp.TableName, id)
			}
		}
	}
	return ""
}

// ExecuteCommand отправляет команду зарегистрированной базе данных
func (service *UserDataService) ExecuteCommand(ctx context.Context, request *api.CommandRequest) (*api.CommandResponse, error) {
	// Проверяем кэш перед выполнением команды
	if cachedResponse, found := service.tryGetFromCache(request); found {
		log.Printf("💾 Using cached response for request: %s", request.RequestId)
		return cachedResponse, nil
	}

	// Выбираем базу данных для выполнения команды
	databases := service.databaseRegistry.ListDatabases()
	if len(databases) == 0 {
		return nil, fmt.Errorf("no databases available")
	}

	// Простая стратегия: выбираем первую доступную БД
	targetDatabase := databases[0].ServiceID

	log.Printf("🔧 Executing command via database %s: %s", targetDatabase, request.RequestId)

	// Отправляем команду выбранной БД
	err := service.databaseRegistry.SendCommandToDatabase(targetDatabase, request)
	if err != nil {
		return nil, fmt.Errorf("failed to send command to database: %v", err)
	}

	// В реальной реализации здесь должно быть ожидание ответа от БД
	// Для простоты возвращаем заглушку с использованием SystemResponse
	return &api.CommandResponse{
		RequestId: request.RequestId,
		Response: &api.CommandResponse_System{
			System: &api.SystemResponse{
				Success: true,
				Message: "Command sent to database for processing",
				Data:    map[string]string{"database": targetDatabase},
			},
		},
	}, nil
}

// tryGetFromCache пытается получить результат из кэша
func (service *UserDataService) tryGetFromCache(request *api.CommandRequest) (*api.CommandResponse, bool) {
	var cacheKey string
	
	// Генерируем ключ кэша в зависимости от типа команды
	switch cmd := request.Command.(type) {
	case *api.CommandRequest_GetOrganization:
		if cmd.GetOrganization != nil {
			// Получаем идентификатор из oneof
			switch identifier := cmd.GetOrganization.Identifier.(type) {
			case *api.GetOrganizationRequest_Id:
				cacheKey = fmt.Sprintf("org:%d", identifier.Id)
			case *api.GetOrganizationRequest_Inn:
				cacheKey = fmt.Sprintf("org:inn:%s", identifier.Inn)
			}
		}
	case *api.CommandRequest_GetUser:
		if cmd.GetUser != nil {
			// Получаем идентификатор из oneof
			switch identifier := cmd.GetUser.Identifier.(type) {
			case *api.GetUserRequest_Id:
				cacheKey = fmt.Sprintf("user:%d", identifier.Id)
			case *api.GetUserRequest_Email:
				cacheKey = fmt.Sprintf("user:email:%s", identifier.Email)
			}
		}
	case *api.CommandRequest_Get:
		if cmd.Get != nil {
			if cmd.Get.Id != 0 {
				cacheKey = fmt.Sprintf("entity:%s:%d", cmd.Get.TableName, cmd.Get.Id)
			}
		}
	}
	
	if cacheKey == "" {
		return nil, false
	}
	
	if cached, found := service.cache.Get(cacheKey); found {
		if response, ok := cached.(*api.CommandResponse); ok {
			return response, true
		}
	}
	
	return nil, false
}

// Методы DataService - теперь они используют ExecuteCommand для отправки команд БД
func (service *UserDataService) GetOrganization(ctx context.Context, request *api.GetOrganizationRequest) (*api.OrganizationResponse, error) {
	command := &api.CommandRequest{
		RequestId: fmt.Sprintf("org_%d", time.Now().UnixNano()),
		Command: &api.CommandRequest_GetOrganization{
			GetOrganization: request,
		},
	}

	response, err := service.ExecuteCommand(ctx, command)
	if err != nil {
		return nil, err
	}

	// Преобразуем ответ
	if orgResponse := response.GetOrganization(); orgResponse != nil {
		return orgResponse, nil
	}

	return nil, fmt.Errorf("invalid response type")
}

func (service *UserDataService) GetUser(ctx context.Context, request *api.GetUserRequest) (*api.UserResponse, error) {
	command := &api.CommandRequest{
		RequestId: fmt.Sprintf("user_%d", time.Now().UnixNano()),
		Command: &api.CommandRequest_GetUser{
			GetUser: request,
		},
	}

	response, err := service.ExecuteCommand(ctx, command)
	if err != nil {
		return nil, err
	}

	if userResponse := response.GetUser(); userResponse != nil {
		return userResponse, nil
	}

	return nil, fmt.Errorf("invalid response type")
}

// GetCacheMetrics возвращает метрики кэша (для мониторинга)
func (service *UserDataService) GetCacheMetrics() string {
	if metricsCache, ok := service.cache.(cache.CacheWithMetrics); ok {
		metrics := metricsCache.GetMetrics()
		return metrics.String()
	}
	
	// Если кэш без метрик, возвращаем базовую информацию
	l1, l2, l3, total := service.cache.GetStats()
	return fmt.Sprintf("Cache Stats: Level1=%d, Level2=%d, Level3=%d, Total=%d/%d", 
		l1, l2, l3, total, service.cache.MaxSize())
}

// ClearCache очищает кэш
func (service *UserDataService) ClearCache() {
	service.cache.Clear()
	log.Printf("🗑️ Cache cleared")
}

// RemoveFromCache удаляет конкретный элемент из кэша
func (service *UserDataService) RemoveFromCache(key string) {
	service.cache.Remove(key)
	log.Printf("🗑️ Cache item removed: %s", key)
}

// loadTLSCredentials загружает TLS сертификаты для сервера
func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Загружаем сертификат сервера
	serverCertificate, serverError := tls.LoadX509KeyPair("certs/mainservice/mainservice-fullchain.crt", "certs/mainservice/mainservice.key")
	if serverError != nil {
		return nil, fmt.Errorf("failed to load server certificates: %v", serverError)
	}

	// Загружаем CA сертификат
	caCertificate, caError := os.ReadFile("certs/ca/ca.crt")
	if caError != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %v", caError)
	}

	certificatePool := x509.NewCertPool()
	if !certificatePool.AppendCertsFromPEM(caCertificate) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	// Настраиваем TLS конфигурацию с правильной верификацией
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{serverCertificate},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    certificatePool,
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}

func main() {
	// Загружаем TLS credentials
	tlsCredentials, credentialsError := loadTLSCredentials()
	if credentialsError != nil {
		log.Fatalf("❌ Failed to load TLS credentials: %v", credentialsError)
	}

	// Создаем gRPC сервер с TLS
	grpcServer := grpc.NewServer(
		grpc.Creds(tlsCredentials),
	)

	userDataService := NewUserDataService()
	
	// Регистрируем оба сервиса
	api.RegisterDataServiceServer(grpcServer, userDataService)
	api.RegisterDatabaseServiceServer(grpcServer, userDataService)

	// Запускаем сервер
	listener, listenerError := net.Listen("tcp", ":5051")
	if listenerError != nil {
		log.Fatalf("❌ Failed to listen: %v", listenerError)
	}

	log.Println("🔐 User Data Service (gRPC Server with mTLS) running on :5051")
	log.Println("   TLS: Enabled (mutual authentication required)")
	log.Println("   Database authentication: Certificate-based (DNS Names)")
	log.Println("   Cache: FIFO3 with metrics enabled")
	log.Println("   Available commands:")
	log.Println("   - GetOrganization, GetUser, CreateUser, ListOrganizations, etc.")
	log.Println("   Registered services: DataService, DatabaseService")

	// Запускаем горутину для логирования метрик кэша
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		
		for range ticker.C {
			metrics := userDataService.GetCacheMetrics()
			log.Printf("📊 Cache Metrics: %s", metrics)
		}
	}()

	if serveError := grpcServer.Serve(listener); serveError != nil {
		log.Fatalf("❌ Failed to serve: %v", serveError)
	}
}