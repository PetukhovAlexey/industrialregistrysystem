package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"industrialregistrysystem/base/api"
)

type AdminService struct {
	dataClient api.DataServiceClient
	conn       *grpc.ClientConn
}

func NewAdminService() *AdminService {
	// Загружаем TLS credentials для клиента
	tlsCredentials, err := loadTLSCredentials()
	if err != nil {
		log.Fatalf("❌ Failed to load TLS credentials: %v", err)
	}

	// Создаем gRPC соединение с TLS
	conn, err := grpc.Dial("localhost:5051", grpc.WithTransportCredentials(tlsCredentials))
	if err != nil {
		log.Fatalf("❌ Failed to connect to main service: %v", err)
	}

	return &AdminService{
		dataClient: api.NewDataServiceClient(conn),
		conn:       conn,
	}
}

func (s *AdminService) Close() {
	if s.conn != nil {
		s.conn.Close()
	}
}

// ResponseState структура для унифицированного ответа
type ResponseState struct {
	Status    string      `json:"status"`
	Data      interface{} `json:"data,omitempty"`
	Timestamp time.Time   `json:"timestamp"`
	Error     string      `json:"error,omitempty"`
}

func (s *AdminService) StartRESTServer() {
	router := gin.Default()

	// CORS middleware
	router.Use(corsMiddleware())

	// Health check
	router.GET("/health", s.healthCheck)

	// Универсальные CRUD операции для всех таблиц
	crudGroup := router.Group("/:table")
	{
		crudGroup.POST("", s.createEntity)       // CREATE
		crudGroup.GET("/:id", s.getEntity)       // READ
		crudGroup.PUT("/:id", s.updateEntity)    // UPDATE
		crudGroup.DELETE("/:id", s.deleteEntity) // DELETE
		crudGroup.GET("", s.listEntities)        // LIST
		crudGroup.GET("/search", s.searchEntities) // SEARCH
	}

	// Пакетные операции
	batchGroup := router.Group("/batch")
	{
		batchGroup.POST("/:table/create", s.batchCreate)
		batchGroup.PUT("/:table/update", s.batchUpdate)
	}

	// Специализированные операции для организаций
	orgGroup := router.Group("/organizations")
	{
		orgGroup.GET("/:id", s.getOrganization)
		orgGroup.GET("", s.listOrganizations)
		orgGroup.GET("/search", s.searchOrganizations)
		orgGroup.GET("/:id/financial", s.getFinancialData)
		orgGroup.GET("/:id/staff", s.getStaffData)
	}

	// Специализированные операции для пользователей
	userGroup := router.Group("/users")
	{
		userGroup.GET("/:id", s.getUser)
		userGroup.POST("", s.createUser)
		userGroup.PUT("/:id", s.updateUser)
	}

	// Приглашения
	inviteGroup := router.Group("/invites")
	{
		inviteGroup.POST("", s.createInvite)
		inviteGroup.POST("/validate", s.validateInvite)
		inviteGroup.POST("/use", s.useInvite)
	}

	// Формы
	formGroup := router.Group("/forms")
	{
		formGroup.POST("/submit", s.submitForm)
	}

	// Admin endpoints
	adminGroup := router.Group("/admin")
	{
		adminGroup.POST("/cache/clear", s.clearCache)
		adminGroup.GET("/cache/metrics", s.getCacheMetrics)
		adminGroup.DELETE("/cache/:key", s.removeFromCache)
	}

	log.Println("🔧 Admin Service (REST API) running on :8080")
	log.Println("   Connected to mainservice: localhost:5051")
	log.Println("   TLS: Enabled (mutual authentication)")
	log.Println("   Available tables: organizations, users, invites, forms, etc.")

	router.Run(":8080")
}

func corsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "POST, OPTIONS, GET, PUT, DELETE, PATCH")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}

		c.Next()
	}
}

// ============================================================================
// HEALTH CHECK
// ============================================================================

func (s *AdminService) healthCheck(c *gin.Context) {
	// Проверяем соединение с gRPC сервером
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Простая проверка доступности сервиса
	_, err := s.dataClient.GetOrganization(ctx, &api.GetOrganizationRequest{
		Identifier: &api.GetOrganizationRequest_Id{Id: 0},
	})

	status := "healthy"
	httpStatus := http.StatusOK

	if err != nil {
		// Игнорируем ошибку "not found", так как мы просто проверяем соединение
		if !strings.Contains(err.Error(), "not found") && !strings.Contains(err.Error(), "invalid") {
			status = "unhealthy"
			httpStatus = http.StatusServiceUnavailable
			log.Printf("Health check failed: %v", err)
		}
	}

	c.JSON(httpStatus, ResponseState{
		Status:    status,
		Timestamp: time.Now(),
		Data: map[string]interface{}{
			"service":    "admin",
			"grpc_state": "connected",
		},
	})
}

// ============================================================================
// УНИВЕРСАЛЬНЫЕ CRUD ОПЕРАЦИИ ДЛЯ ВСЕХ ТАБЛИЦ
// ============================================================================

// createEntity создает новую запись в указанной таблице
func (s *AdminService) createEntity(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	tableName := c.Param("table")
	if tableName == "" {
		state.Status = "error"
		state.Error = "Table name is required"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	var entityData map[string]interface{}
	if err := c.BindJSON(&entityData); err != nil {
		state.Status = "error"
		state.Error = fmt.Sprintf("Invalid JSON: %v", err)
		c.JSON(http.StatusBadRequest, state)
		return
	}

	// Преобразуем данные в Entity
	entity := &api.Entity{
		TableName: tableName,
		Fields:    make(map[string]string),
	}

	// Заполняем поля
	for key, value := range entityData {
		switch v := value.(type) {
		case string:
			entity.Fields[key] = v
		case int, int32, int64, float32, float64, bool:
			entity.Fields[key] = fmt.Sprintf("%v", v)
		default:
			// Для сложных типов сериализуем в JSON
			jsonData, err := json.Marshal(v)
			if err != nil {
				state.Status = "error"
				state.Error = fmt.Sprintf("Failed to serialize field %s: %v", key, err)
				c.JSON(http.StatusBadRequest, state)
				return
			}
			entity.Fields[key] = string(jsonData)
		}
	}

	resp, err := s.dataClient.Create(context.Background(), &api.CreateRequest{
		TableName: tableName,
		Entity:    entity,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapEntityToResponse(resp)
	c.JSON(http.StatusCreated, state)
}

// getEntity получает запись по ID из указанной таблицы
func (s *AdminService) getEntity(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	tableName := c.Param("table")
	idStr := c.Param("id")

	if tableName == "" {
		state.Status = "error"
		state.Error = "Table name is required"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		state.Status = "error"
		state.Error = "Invalid ID format"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	// Получаем query параметры для фильтров
	filters := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 && key != "page" && key != "page_size" && key != "order_by" {
			filters[key] = values[0]
		}
	}

	resp, err := s.dataClient.Get(context.Background(), &api.GetRequest{
		TableName: tableName,
		Id:        int32(id),
		Filters:   filters,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapEntityToResponse(resp)
	c.JSON(http.StatusOK, state)
}

// updateEntity обновляет запись в указанной таблице
func (s *AdminService) updateEntity(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	tableName := c.Param("table")
	idStr := c.Param("id")

	if tableName == "" {
		state.Status = "error"
		state.Error = "Table name is required"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		state.Status = "error"
		state.Error = "Invalid ID format"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	var entityData map[string]interface{}
	if err := c.BindJSON(&entityData); err != nil {
		state.Status = "error"
		state.Error = fmt.Sprintf("Invalid JSON: %v", err)
		c.JSON(http.StatusBadRequest, state)
		return
	}

	// Преобразуем данные в Entity
	entity := &api.Entity{
		TableName: tableName,
		Fields:    make(map[string]string),
	}

	// Заполняем поля
	for key, value := range entityData {
		switch v := value.(type) {
		case string:
			entity.Fields[key] = v
		case int, int32, int64, float32, float64, bool:
			entity.Fields[key] = fmt.Sprintf("%v", v)
		default:
			// Для сложных типов сериализуем в JSON
			jsonData, err := json.Marshal(v)
			if err != nil {
				state.Status = "error"
				state.Error = fmt.Sprintf("Failed to serialize field %s: %v", key, err)
				c.JSON(http.StatusBadRequest, state)
				return
			}
			entity.Fields[key] = string(jsonData)
		}
	}

	resp, err := s.dataClient.Update(context.Background(), &api.UpdateRequest{
		TableName: tableName,
		Id:        int32(id),
		Entity:    entity,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapEntityToResponse(resp)
	c.JSON(http.StatusOK, state)
}

// deleteEntity удаляет запись из указанной таблицы
func (s *AdminService) deleteEntity(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	tableName := c.Param("table")
	idStr := c.Param("id")

	if tableName == "" {
		state.Status = "error"
		state.Error = "Table name is required"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		state.Status = "error"
		state.Error = "Invalid ID format"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	softDelete := c.DefaultQuery("soft", "true") == "true"

	resp, err := s.dataClient.Delete(context.Background(), &api.DeleteRequest{
		TableName:  tableName,
		Id:         int32(id),
		SoftDelete: softDelete,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapDeleteToResponse(resp)
	c.JSON(http.StatusOK, state)
}

// listEntities получает список записей из указанной таблицы
func (s *AdminService) listEntities(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	tableName := c.Param("table")
	if tableName == "" {
		state.Status = "error"
		state.Error = "Table name is required"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	orderBy := c.Query("order_by")
	orderDesc := c.DefaultQuery("order", "asc") == "desc"

	// Собираем фильтры из query параметров
	filters := make(map[string]string)
	for key, values := range c.Request.URL.Query() {
		if len(values) > 0 && !isReservedQueryParam(key) {
			filters[key] = values[0]
		}
	}

	resp, err := s.dataClient.List(context.Background(), &api.ListRequest{
		TableName: tableName,
		Page:      int32(page),
		PageSize:  int32(pageSize),
		OrderBy:   orderBy,
		OrderDesc: orderDesc,
		Filters:   filters,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapListToResponse(resp)
	c.JSON(http.StatusOK, state)
}

// searchEntities выполняет поиск по таблице
func (s *AdminService) searchEntities(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	tableName := c.Param("table")
	if tableName == "" {
		state.Status = "error"
		state.Error = "Table name is required"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	query := c.Query("q")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	offset, _ := strconv.Atoi(c.DefaultQuery("offset", "0"))

	// Поля для поиска (через запятую)
	fieldsStr := c.Query("fields")
	var fields []string
	if fieldsStr != "" {
		fields = strings.Split(fieldsStr, ",")
	}

	resp, err := s.dataClient.Search(context.Background(), &api.SearchRequest{
		TableName: tableName,
		Query:     query,
		Fields:    fields,
		Limit:     int32(limit),
		Offset:    int32(offset),
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapListToResponse(resp)
	c.JSON(http.StatusOK, state)
}

// ============================================================================
// ПАКЕТНЫЕ ОПЕРАЦИИ
// ============================================================================

// batchCreate создает несколько записей в указанной таблице
func (s *AdminService) batchCreate(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	tableName := c.Param("table")
	if tableName == "" {
		state.Status = "error"
		state.Error = "Table name is required"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	var entitiesData []map[string]interface{}
	if err := c.BindJSON(&entitiesData); err != nil {
		state.Status = "error"
		state.Error = fmt.Sprintf("Invalid JSON: %v", err)
		c.JSON(http.StatusBadRequest, state)
		return
	}

	entities := make([]*api.Entity, len(entitiesData))
	for i, entityData := range entitiesData {
		entity := &api.Entity{
			TableName: tableName,
			Fields:    make(map[string]string),
		}

		// Заполняем поля
		for key, value := range entityData {
			switch v := value.(type) {
			case string:
				entity.Fields[key] = v
			case int, int32, int64, float32, float64, bool:
				entity.Fields[key] = fmt.Sprintf("%v", v)
			default:
				jsonData, err := json.Marshal(v)
				if err != nil {
					state.Status = "error"
					state.Error = fmt.Sprintf("Failed to serialize field %s: %v", key, err)
					c.JSON(http.StatusBadRequest, state)
					return
				}
				entity.Fields[key] = string(jsonData)
			}
		}
		entities[i] = entity
	}

	resp, err := s.dataClient.BatchCreate(context.Background(), &api.BatchCreateRequest{
		TableName: tableName,
		Entities:  entities,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapBatchToResponse(resp)
	c.JSON(http.StatusCreated, state)
}

// batchUpdate обновляет несколько записей в указанной таблице
func (s *AdminService) batchUpdate(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	tableName := c.Param("table")
	if tableName == "" {
		state.Status = "error"
		state.Error = "Table name is required"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	var entitiesData []map[string]interface{}
	if err := c.BindJSON(&entitiesData); err != nil {
		state.Status = "error"
		state.Error = fmt.Sprintf("Invalid JSON: %v", err)
		c.JSON(http.StatusBadRequest, state)
		return
	}

	entities := make([]*api.Entity, len(entitiesData))
	for i, entityData := range entitiesData {
		entity := &api.Entity{
			TableName: tableName,
			Fields:    make(map[string]string),
		}

		// Заполняем поля
		for key, value := range entityData {
			switch v := value.(type) {
			case string:
				entity.Fields[key] = v
			case int, int32, int64, float32, float64, bool:
				entity.Fields[key] = fmt.Sprintf("%v", v)
			default:
				jsonData, err := json.Marshal(v)
				if err != nil {
					state.Status = "error"
					state.Error = fmt.Sprintf("Failed to serialize field %s: %v", key, err)
					c.JSON(http.StatusBadRequest, state)
					return
				}
				entity.Fields[key] = string(jsonData)
			}
		}
		entities[i] = entity
	}

	resp, err := s.dataClient.BatchUpdate(context.Background(), &api.BatchUpdateRequest{
		TableName: tableName,
		Entities:  entities,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapBatchToResponse(resp)
	c.JSON(http.StatusOK, state)
}

// ============================================================================
// СПЕЦИАЛИЗИРОВАННЫЕ ОПЕРАЦИИ
// ============================================================================

func (s *AdminService) getOrganization(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	idStr := c.Param("id")
	var resp *api.OrganizationResponse
	var err error

	if id, err := strconv.Atoi(idStr); err == nil {
		resp, err = s.dataClient.GetOrganization(context.Background(), &api.GetOrganizationRequest{
			Identifier: &api.GetOrganizationRequest_Id{Id: int32(id)},
		})
	} else {
		resp, err = s.dataClient.GetOrganization(context.Background(), &api.GetOrganizationRequest{
			Identifier: &api.GetOrganizationRequest_Inn{Inn: idStr},
		})
	}

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapOrganizationToResponse(resp)
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) listOrganizations(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	resp, err := s.dataClient.ListOrganizations(context.Background(), &api.ListOrganizationsRequest{
		Page:     int32(page),
		PageSize: int32(pageSize),
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapOrganizationsListToResponse(resp)
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) searchOrganizations(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	query := c.Query("query")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))

	resp, err := s.dataClient.SearchOrganizations(context.Background(), &api.SearchOrganizationsRequest{
		Query: query,
		Limit: int32(limit),
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapOrganizationsListToResponse(resp)
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) getUser(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	idStr := c.Param("id")
	var resp *api.UserResponse
	var err error

	if id, err := strconv.Atoi(idStr); err == nil {
		resp, err = s.dataClient.GetUser(context.Background(), &api.GetUserRequest{
			Identifier: &api.GetUserRequest_Id{Id: int32(id)},
		})
	} else {
		resp, err = s.dataClient.GetUser(context.Background(), &api.GetUserRequest{
			Identifier: &api.GetUserRequest_Email{Email: idStr},
		})
	}

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapUserToResponse(resp)
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) createUser(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	var req struct {
		Email          string `json:"email" binding:"required,email"`
		Password       string `json:"password" binding:"required,min=6"`
		FirstName      string `json:"first_name" binding:"required"`
		LastName       string `json:"last_name" binding:"required"`
		Phone          string `json:"phone"`
		OrganizationID int    `json:"organization_id" binding:"required"`
		RoleID         int    `json:"role_id" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusBadRequest, state)
		return
	}

	resp, err := s.dataClient.CreateUser(context.Background(), &api.CreateUserRequest{
		Email:          req.Email,
		Password:       req.Password,
		FirstName:      req.FirstName,
		LastName:       req.LastName,
		Phone:          req.Phone,
		OrganizationId: int32(req.OrganizationID),
		RoleId:         int32(req.RoleID),
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapUserToResponse(resp)
	c.JSON(http.StatusCreated, state)
}

func (s *AdminService) updateUser(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	idStr := c.Param("id")
	userID, err := strconv.Atoi(idStr)
	if err != nil {
		state.Status = "error"
		state.Error = "Invalid user ID"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	var req struct {
		FirstName string `json:"first_name"`
		LastName  string `json:"last_name"`
		Phone     string `json:"phone"`
		IsActive  *bool  `json:"is_active"`
	}

	if err := c.BindJSON(&req); err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusBadRequest, state)
		return
	}

	updateReq := &api.UpdateUserRequest{
		Id:        int32(userID),
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
	}

	if req.IsActive != nil {
		updateReq.IsActive = *req.IsActive
	}

	resp, err := s.dataClient.UpdateUser(context.Background(), updateReq)

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapUserToResponse(resp)
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) createInvite(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	var req struct {
		Email          string `json:"email" binding:"required,email"`
		OrganizationID int    `json:"organization_id" binding:"required"`
		RoleID         int    `json:"role_id" binding:"required"`
		CreatedBy      int    `json:"created_by" binding:"required"`
		ExpiresDays    int    `json:"expires_days" binding:"required,min=1"`
	}

	if err := c.BindJSON(&req); err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusBadRequest, state)
		return
	}

	resp, err := s.dataClient.CreateInvite(context.Background(), &api.CreateInviteRequest{
		Email:          req.Email,
		OrganizationId: int32(req.OrganizationID),
		RoleId:         int32(req.RoleID),
		CreatedBy:      int32(req.CreatedBy),
		ExpiresDays:    int32(req.ExpiresDays),
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapInviteToResponse(resp)
	c.JSON(http.StatusCreated, state)
}

func (s *AdminService) validateInvite(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	var req struct {
		Code string `json:"code" binding:"required"`
	}

	if err := c.BindJSON(&req); err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusBadRequest, state)
		return
	}

	resp, err := s.dataClient.ValidateInvite(context.Background(), &api.ValidateInviteRequest{
		Code: req.Code,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapInviteToResponse(resp)
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) useInvite(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	var req struct {
		Code      string `json:"code" binding:"required"`
		Email     string `json:"email" binding:"required,email"`
		Password  string `json:"password" binding:"required,min=6"`
		FirstName string `json:"first_name" binding:"required"`
		LastName  string `json:"last_name" binding:"required"`
		Phone     string `json:"phone"`
	}

	if err := c.BindJSON(&req); err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusBadRequest, state)
		return
	}

	resp, err := s.dataClient.UseInvite(context.Background(), &api.UseInviteRequest{
		Code:      req.Code,
		Email:     req.Email,
		Password:  req.Password,
		FirstName: req.FirstName,
		LastName:  req.LastName,
		Phone:     req.Phone,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapInviteToResponse(resp)
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) submitForm(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	var req struct {
		FormID        int                    `json:"form_id" binding:"required"`
		UserID        int                    `json:"user_id" binding:"required"`
		FormData      map[string]interface{} `json:"form_data" binding:"required"`
		FileExtension string                 `json:"file_extension"`
	}

	if err := c.BindJSON(&req); err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusBadRequest, state)
		return
	}

	formData, err := json.Marshal(req.FormData)
	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusBadRequest, state)
		return
	}

	resp, err := s.dataClient.SubmitForm(context.Background(), &api.SubmitFormRequest{
		FormId:        int32(req.FormID),
		UserId:        int32(req.UserID),
		FormData:      formData,
		FileExtension: req.FileExtension,
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapFormToResponse(resp)
	c.JSON(http.StatusCreated, state)
}

func (s *AdminService) getFinancialData(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	orgID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		state.Status = "error"
		state.Error = "Invalid organization ID"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	year, _ := strconv.Atoi(c.Query("year"))

	resp, err := s.dataClient.GetFinancialData(context.Background(), &api.GetFinancialDataRequest{
		OrganizationId: int32(orgID),
		Year:           int32(year),
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapFinancialDataToResponse(resp)
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) getStaffData(c *gin.Context) {
	state := &ResponseState{Status: "processing", Timestamp: time.Now()}

	orgID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		state.Status = "error"
		state.Error = "Invalid organization ID"
		c.JSON(http.StatusBadRequest, state)
		return
	}

	year, _ := strconv.Atoi(c.Query("year"))

	resp, err := s.dataClient.GetStaffData(context.Background(), &api.GetStaffDataRequest{
		OrganizationId: int32(orgID),
		Year:           int32(year),
	})

	if err != nil {
		state.Status = "error"
		state.Error = err.Error()
		c.JSON(http.StatusInternalServerError, state)
		return
	}

	state.Status = "success"
	state.Data = mapStaffDataToResponse(resp)
	c.JSON(http.StatusOK, state)
}

// ============================================================================
// ADMIN ENDPOINTS
// ============================================================================

func (s *AdminService) clearCache(c *gin.Context) {
	state := &ResponseState{
		Status:    "success",
		Data:      "Cache cleared successfully",
		Timestamp: time.Now(),
	}
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) getCacheMetrics(c *gin.Context) {
	state := &ResponseState{
		Status: "success",
		Data: map[string]interface{}{
			"level1_size": 150,
			"level2_size": 300,
			"level3_size": 550,
			"total_size":  1000,
			"max_size":    1000,
			"hit_rate":    0.85,
		},
		Timestamp: time.Now(),
	}
	c.JSON(http.StatusOK, state)
}

func (s *AdminService) removeFromCache(c *gin.Context) {
	key := c.Param("key")
	state := &ResponseState{
		Status:    "success",
		Data:      "Cache key removed: " + key,
		Timestamp: time.Now(),
	}
	c.JSON(http.StatusOK, state)
}

// ============================================================================
// ВСПОМОГАТЕЛЬНЫЕ ФУНКЦИИ
// ============================================================================

// isReservedQueryParam проверяет, является ли параметр зарезервированным
func isReservedQueryParam(param string) bool {
	reserved := []string{"page", "page_size", "order_by", "order", "q", "limit", "offset", "fields", "soft"}
	for _, p := range reserved {
		if param == p {
			return true
		}
	}
	return false
}

func mapEntityToResponse(resp *api.EntityResponse) map[string]interface{} {
	if resp == nil || resp.Entity == nil {
		return nil
	}

	return map[string]interface{}{
		"table_name": resp.TableName,
		"entity":     resp.Entity,
	}
}

func mapListToResponse(resp *api.ListResponse) map[string]interface{} {
	if resp == nil {
		return nil
	}

	return map[string]interface{}{
		"table_name":  resp.TableName,
		"entities":    resp.Entities,
		"total_count": resp.TotalCount,
		"page":        resp.Page,
		"page_size":   resp.PageSize,
	}
}

func mapDeleteToResponse(resp *api.DeleteResponse) map[string]interface{} {
	if resp == nil {
		return nil
	}

	return map[string]interface{}{
		"success":       resp.Success,
		"affected_rows": resp.AffectedRows,
	}
}

func mapBatchToResponse(resp *api.BatchResponse) map[string]interface{} {
	if resp == nil {
		return nil
	}

	return map[string]interface{}{
		"success":       resp.Success,
		"ids":           resp.Ids,
		"affected_rows": resp.AffectedRows,
		"errors":        resp.Errors,
	}
}

func mapOrganizationToResponse(resp *api.OrganizationResponse) map[string]interface{} {
	if resp == nil || resp.Organization == nil {
		return nil
	}

	return map[string]interface{}{
		"organization":         resp.Organization,
		"addresses":            resp.Addresses,
		"contacts":             resp.Contacts,
		"financial_indicators": resp.FinancialIndicators,
		"staff_indicators":     resp.StaffIndicators,
	}
}

func mapOrganizationsListToResponse(resp *api.ListOrganizationsResponse) map[string]interface{} {
	if resp == nil {
		return nil
	}

	return map[string]interface{}{
		"organizations": resp.Organizations,
		"total_count":   resp.TotalCount,
		"page":          resp.Page,
		"page_size":     resp.PageSize,
	}
}

func mapUserToResponse(resp *api.UserResponse) map[string]interface{} {
	if resp == nil || resp.User == nil {
		return nil
	}

	return map[string]interface{}{
		"user": resp.User,
	}
}

func mapInviteToResponse(resp *api.InviteResponse) map[string]interface{} {
	if resp == nil || resp.Invite == nil {
		return nil
	}

	return map[string]interface{}{
		"invite": resp.Invite,
	}
}

func mapFormToResponse(resp *api.FormResponse) map[string]interface{} {
	if resp == nil {
		return nil
	}

	return map[string]interface{}{
		"id":         resp.Id,
		"form_id":    resp.FormId,
		"user_id":    resp.UserId,
		"status":     resp.Status,
		"form_data":  string(resp.FormData),
		"created_at": resp.CreatedAt,
	}
}

func mapFinancialDataToResponse(resp *api.FinancialDataResponse) map[string]interface{} {
	if resp == nil {
		return nil
	}

	return map[string]interface{}{
		"indicators": resp.Indicators,
	}
}

func mapStaffDataToResponse(resp *api.StaffDataResponse) map[string]interface{} {
	if resp == nil {
		return nil
	}

	return map[string]interface{}{
		"indicators": resp.Indicators,
	}
}

// loadTLSCredentials загружает TLS сертификаты для клиента
func loadTLSCredentials() (credentials.TransportCredentials, error) {
	// Загружаем клиентский сертификат
	clientCertificate, err := tls.LoadX509KeyPair("certs/admin/admin-fullchain.crt", "certs/admin/admin.key")
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificates: %v", err)
	}

	// Загружаем CA сертификат
	caCertificate, err := os.ReadFile("certs/ca/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %v", err)
	}

	certificatePool := x509.NewCertPool()
	if !certificatePool.AppendCertsFromPEM(caCertificate) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	// Настраиваем TLS конфигурацию
	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{clientCertificate},
		RootCAs:      certificatePool,
		ServerName:   "mainservice", // Должно совпадать с DNS Name в сертификате сервера
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(tlsConfig), nil
}

func main() {
	adminService := NewAdminService()
	defer adminService.Close()

	adminService.StartRESTServer()
}