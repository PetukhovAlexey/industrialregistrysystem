package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/lib/pq"
	"industrialregistrysystem/base/api"
)

type DataService struct {
	db *sql.DB
}

func NewDataService() *DataService {
	db, err := sql.Open("postgres", "host=192.168.1.137 user=myuser password=mypassword dbname=mydatabase sslmode=disable port=5433")
	if err != nil {
		log.Fatal(err)
	}
	
	// Настройка пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)
	
	// Проверяем подключение
	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}
	
	return &DataService{db: db}
}

// Create - универсальное создание записи
func (dataService *DataService) Create(ctx context.Context, createRequest *api.CreateRequest) (*api.EntityResponse, error) {
	// Определяем таблицу и поля для вставки
	tableName := createRequest.TableName
	entity := createRequest.Entity
	
	// Формируем SQL запрос динамически
	columns := ""
	placeholders := ""
	values := []interface{}{}
	parameterIndex := 1
	
	for fieldName, fieldValue := range entity.Fields {
		if columns != "" {
			columns += ", "
			placeholders += ", "
		}
		columns += fieldName
		placeholders += "$" + fmt.Sprintf("%d", parameterIndex)
		values = append(values, fieldValue)
		parameterIndex++
	}
	
	query := "INSERT INTO " + tableName + " (" + columns + ") VALUES (" + placeholders + ") RETURNING id"
	
	var id int32
	err := dataService.db.QueryRowContext(ctx, query, values...).Scan(&id)
	if err != nil {
		return nil, err
	}
	
	// Возвращаем созданную запись
	return dataService.Get(ctx, &api.GetRequest{
		TableName: tableName,
		Id:        id,
	})
}

// Get - универсальное получение записи по ID
func (dataService *DataService) Get(ctx context.Context, getRequest *api.GetRequest) (*api.EntityResponse, error) {
	query := "SELECT * FROM " + getRequest.TableName + " WHERE id = $1 AND destroyed = false"
	
	rows, err := dataService.db.QueryContext(ctx, query, getRequest.Id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	
	// Создаем срезы для значений
	values := make([]interface{}, len(columns))
	valuePointers := make([]interface{}, len(columns))
	for i := range values {
		valuePointers[i] = &values[i]
	}
	
	if !rows.Next() {
		return nil, sql.ErrNoRows
	}
	
	err = rows.Scan(valuePointers...)
	if err != nil {
		return nil, err
	}
	
	// Преобразуем результат в Entity с правильным преобразованием типов
	entity := &api.Entity{
		Fields: make(map[string]string),
	}
	
	for i, columnName := range columns {
		if values[i] != nil {
			// Правильное преобразование типов
			switch v := values[i].(type) {
			case []byte:
				entity.Fields[columnName] = string(v)
			case string:
				entity.Fields[columnName] = v
			case int64:
				entity.Fields[columnName] = fmt.Sprintf("%d", v)
			case float64:
				entity.Fields[columnName] = fmt.Sprintf("%f", v)
			case bool:
				entity.Fields[columnName] = fmt.Sprintf("%t", v)
			case time.Time:
				entity.Fields[columnName] = v.Format(time.RFC3339)
			default:
				entity.Fields[columnName] = fmt.Sprintf("%v", v)
			}
		} else {
			entity.Fields[columnName] = ""
		}
	}
	
	return &api.EntityResponse{
		TableName: getRequest.TableName,
		Entity:    entity,
	}, nil
}

// Update - универсальное обновление записи
func (dataService *DataService) Update(ctx context.Context, updateRequest *api.UpdateRequest) (*api.EntityResponse, error) {
	tableName := updateRequest.TableName
	entity := updateRequest.Entity
	
	// Формируем SET часть запроса
	setClause := ""
	values := []interface{}{}
	parameterIndex := 1
	
	for fieldName, fieldValue := range entity.Fields {
		if setClause != "" {
			setClause += ", "
		}
		setClause += fieldName + " = $" + fmt.Sprintf("%d", parameterIndex)
		values = append(values, fieldValue)
		parameterIndex++
	}
	
	// Добавляем ID в конец
	values = append(values, updateRequest.Id)
	
	query := "UPDATE " + tableName + " SET " + setClause + ", updated_at = NOW() WHERE id = $" + fmt.Sprintf("%d", parameterIndex) + " AND destroyed = false"
	
	_, err := dataService.db.ExecContext(ctx, query, values...)
	if err != nil {
		return nil, err
	}
	
	return dataService.Get(ctx, &api.GetRequest{
		TableName: tableName,
		Id:        updateRequest.Id,
	})
}

// Delete - универсальное удаление записи
func (dataService *DataService) Delete(ctx context.Context, deleteRequest *api.DeleteRequest) (*api.DeleteResponse, error) {
	var query string
	var result sql.Result
	var err error
	
	if deleteRequest.SoftDelete {
		query = "UPDATE " + deleteRequest.TableName + " SET destroyed = true, updated_at = NOW() WHERE id = $1"
	} else {
		query = "DELETE FROM " + deleteRequest.TableName + " WHERE id = $1"
	}
	
	result, err = dataService.db.ExecContext(ctx, query, deleteRequest.Id)
	if err != nil {
		return nil, err
	}
	
	affectedRows, _ := result.RowsAffected()
	
	return &api.DeleteResponse{
		Success:      affectedRows > 0,
		AffectedRows: int32(affectedRows),
	}, nil
}

// List - универсальное получение списка записей
func (dataService *DataService) List(ctx context.Context, listRequest *api.ListRequest) (*api.ListResponse, error) {
	// Базовый запрос
	query := "SELECT * FROM " + listRequest.TableName + " WHERE destroyed = false"
	countQuery := "SELECT COUNT(*) FROM " + listRequest.TableName + " WHERE destroyed = false"
	
	values := []interface{}{}
	paramIndex := 1
	
	// Добавляем фильтры
	if len(listRequest.Filters) > 0 {
		query += " AND ("
		countQuery += " AND ("
		
		first := true
		for fieldName, fieldValue := range listRequest.Filters {
			if !first {
				query += " AND "
				countQuery += " AND "
			}
			query += fieldName + " = $" + fmt.Sprintf("%d", paramIndex)
			countQuery += fieldName + " = $" + fmt.Sprintf("%d", paramIndex)
			values = append(values, fieldValue)
			paramIndex++
			first = false
		}
		
		query += ")"
		countQuery += ")"
	}
	
	// Добавляем сортировку
	if listRequest.OrderBy != "" {
		query += " ORDER BY " + listRequest.OrderBy
		if listRequest.OrderDesc {
			query += " DESC"
		}
	}
	
	// Добавляем пагинацию
	query += " LIMIT $" + fmt.Sprintf("%d", paramIndex) + " OFFSET $" + fmt.Sprintf("%d", paramIndex+1)
	values = append(values, listRequest.PageSize, (listRequest.Page-1)*listRequest.PageSize)
	
	// Выполняем запрос на получение данных
	rows, err := dataService.db.QueryContext(ctx, query, values...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	
	var entities []*api.Entity
	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		rowValuePointers := make([]interface{}, len(columns))
		for i := range rowValues {
			rowValuePointers[i] = &rowValues[i]
		}
		
		err = rows.Scan(rowValuePointers...)
		if err != nil {
			return nil, err
		}
		
		entity := &api.Entity{
			Fields: make(map[string]string),
		}
		
		for i, columnName := range columns {
			if rowValues[i] != nil {
				switch v := rowValues[i].(type) {
				case []byte:
					entity.Fields[columnName] = string(v)
				case string:
					entity.Fields[columnName] = v
				case int64:
					entity.Fields[columnName] = fmt.Sprintf("%d", v)
				case float64:
					entity.Fields[columnName] = fmt.Sprintf("%f", v)
				case bool:
					entity.Fields[columnName] = fmt.Sprintf("%t", v)
				case time.Time:
					entity.Fields[columnName] = v.Format(time.RFC3339)
				default:
					entity.Fields[columnName] = fmt.Sprintf("%v", v)
				}
			} else {
				entity.Fields[columnName] = ""
			}
		}
		
		entities = append(entities, entity)
	}
	
	// Получаем общее количество
	var totalCount int32
	countValues := values[:len(values)-2] // Убираем LIMIT и OFFSET
	if len(countValues) > 0 {
		err = dataService.db.QueryRowContext(ctx, countQuery, countValues...).Scan(&totalCount)
	} else {
		err = dataService.db.QueryRowContext(ctx, countQuery).Scan(&totalCount)
	}
	if err != nil {
		return nil, err
	}
	
	return &api.ListResponse{
		TableName:  listRequest.TableName,
		Entities:   entities,
		TotalCount: totalCount,
		Page:       listRequest.Page,
		PageSize:   listRequest.PageSize,
	}, nil
}

// Search - универсальный поиск
func (dataService *DataService) Search(ctx context.Context, searchRequest *api.SearchRequest) (*api.ListResponse, error) {
	if len(searchRequest.Fields) == 0 {
		return nil, fmt.Errorf("search fields are required")
	}
	
	query := "SELECT * FROM " + searchRequest.TableName + " WHERE destroyed = false AND ("
	countQuery := "SELECT COUNT(*) FROM " + searchRequest.TableName + " WHERE destroyed = false AND ("
	
	searchPattern := "%" + searchRequest.Query + "%"
	values := []interface{}{}
	
	for fieldIndex, fieldName := range searchRequest.Fields {
		if fieldIndex > 0 {
			query += " OR "
			countQuery += " OR "
		}
		query += fieldName + " ILIKE $" + fmt.Sprintf("%d", fieldIndex+1)
		countQuery += fieldName + " ILIKE $" + fmt.Sprintf("%d", fieldIndex+1)
		values = append(values, searchPattern)
	}
	
	query += ")"
	countQuery += ")"
	
	// Добавляем лимит и оффсет
	if searchRequest.Limit > 0 {
		query += " LIMIT $" + fmt.Sprintf("%d", len(values)+1)
		values = append(values, searchRequest.Limit)
	}
	
	if searchRequest.Offset > 0 {
		query += " OFFSET $" + fmt.Sprintf("%d", len(values)+1)
		values = append(values, searchRequest.Offset)
	}
	
	// Выполняем запрос на получение данных
	rows, err := dataService.db.QueryContext(ctx, query, values...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	
	var entities []*api.Entity
	for rows.Next() {
		rowValues := make([]interface{}, len(columns))
		rowValuePointers := make([]interface{}, len(columns))
		for i := range rowValues {
			rowValuePointers[i] = &rowValues[i]
		}
		
		err = rows.Scan(rowValuePointers...)
		if err != nil {
			return nil, err
		}
		
		entity := &api.Entity{
			Fields: make(map[string]string),
		}
		
		for i, columnName := range columns {
			if rowValues[i] != nil {
				switch v := rowValues[i].(type) {
				case []byte:
					entity.Fields[columnName] = string(v)
				case string:
					entity.Fields[columnName] = v
				case int64:
					entity.Fields[columnName] = fmt.Sprintf("%d", v)
				case float64:
					entity.Fields[columnName] = fmt.Sprintf("%f", v)
				case bool:
					entity.Fields[columnName] = fmt.Sprintf("%t", v)
				case time.Time:
					entity.Fields[columnName] = v.Format(time.RFC3339)
				default:
					entity.Fields[columnName] = fmt.Sprintf("%v", v)
				}
			} else {
				entity.Fields[columnName] = ""
			}
		}
		
		entities = append(entities, entity)
	}
	
	// Получаем общее количество
	var totalCount int32
	err = dataService.db.QueryRowContext(ctx, countQuery, values[:len(searchRequest.Fields)]...).Scan(&totalCount)
	if err != nil {
		return nil, err
	}
	
	return &api.ListResponse{
		TableName:  searchRequest.TableName,
		Entities:   entities,
		TotalCount: totalCount,
		Page:       1,
		PageSize:   int32(len(entities)),
	}, nil
}

// BatchCreate - пакетное создание записей
func (dataService *DataService) BatchCreate(ctx context.Context, batchCreateRequest *api.BatchCreateRequest) (*api.BatchResponse, error) {
	transaction, err := dataService.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer transaction.Rollback()
	
	var ids []int32
	var errors []string
	
	for _, entity := range batchCreateRequest.Entities {
		columns := ""
		placeholders := ""
		values := []interface{}{}
		parameterIndex := 1
		
		for fieldName, fieldValue := range entity.Fields {
			if columns != "" {
				columns += ", "
				placeholders += ", "
			}
			columns += fieldName
			placeholders += "$" + fmt.Sprintf("%d", parameterIndex)
			values = append(values, fieldValue)
			parameterIndex++
		}
		
		query := "INSERT INTO " + batchCreateRequest.TableName + " (" + columns + ") VALUES (" + placeholders + ") RETURNING id"
		
		var id int32
		err := transaction.QueryRowContext(ctx, query, values...).Scan(&id)
		if err != nil {
			errors = append(errors, err.Error())
		} else {
			ids = append(ids, id)
		}
	}
	
	if err := transaction.Commit(); err != nil {
		return nil, err
	}
	
	return &api.BatchResponse{
		Success:      len(errors) == 0,
		Ids:          ids,
		AffectedRows: int32(len(ids)),
		Errors:       errors,
	}, nil
}

// BatchUpdate - пакетное обновление записей
func (dataService *DataService) BatchUpdate(ctx context.Context, batchUpdateRequest *api.BatchUpdateRequest) (*api.BatchResponse, error) {
	transaction, err := dataService.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer transaction.Rollback()
	
	var ids []int32
	var errors []string
	affectedRows := int32(0)
	
	for _, entity := range batchUpdateRequest.Entities {
		// Предполагаем, что ID есть в fields
		idString, exists := entity.Fields["id"]
		if !exists {
			errors = append(errors, "ID field is required for batch update")
			continue
		}
		
		var id int32
		fmt.Sscanf(idString, "%d", &id)
		
		setClause := ""
		values := []interface{}{}
		parameterIndex := 1
		
		for fieldName, fieldValue := range entity.Fields {
			if fieldName == "id" {
				continue
			}
			if setClause != "" {
				setClause += ", "
			}
			setClause += fieldName + " = $" + fmt.Sprintf("%d", parameterIndex)
			values = append(values, fieldValue)
			parameterIndex++
		}
		
		values = append(values, id)
		query := "UPDATE " + batchUpdateRequest.TableName + " SET " + setClause + ", updated_at = NOW() WHERE id = $" + fmt.Sprintf("%d", parameterIndex) + " AND destroyed = false"
		
		result, err := transaction.ExecContext(ctx, query, values...)
		if err != nil {
			errors = append(errors, err.Error())
		} else {
			rowsAffected, _ := result.RowsAffected()
			affectedRows += int32(rowsAffected)
			ids = append(ids, id)
		}
	}
	
	if err := transaction.Commit(); err != nil {
		return nil, err
	}
	
	return &api.BatchResponse{
		Success:      len(errors) == 0,
		Ids:          ids,
		AffectedRows: affectedRows,
		Errors:       errors,
	}, nil
}

// ListOrganizations - получение списка организаций
func (dataService *DataService) ListOrganizations(ctx context.Context, req *api.ListOrganizationsRequest) (*api.ListOrganizationsResponse, error) {
	query := `SELECT id, inn, name, full_name, spark_status, internal_status, final_status, 
					 registration_date, added_to_registry_date, has_special_status, 
					 is_systemically_important, msp_status, created_at, updated_at 
			  FROM active_organizations`
	
	countQuery := "SELECT COUNT(*) FROM active_organizations"
	
	args := []interface{}{}
	
	if req.Filter != "" {
		query += " WHERE name ILIKE $1 OR inn ILIKE $1"
		countQuery += " WHERE name ILIKE $1 OR inn ILIKE $1"
		args = append(args, "%"+req.Filter+"%")
	}
	
	query += " ORDER BY name LIMIT $2 OFFSET $3"
	args = append(args, req.PageSize, (req.Page-1)*req.PageSize)
	
	rows, err := dataService.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var organizations []*api.Organization
	for rows.Next() {
		var org api.Organization
		err := rows.Scan(
			&org.Id, &org.Inn, &org.Name, &org.FullName, &org.SparkStatus, &org.InternalStatus,
			&org.FinalStatus, &org.RegistrationDate, &org.AddedToRegistryDate, &org.HasSpecialStatus,
			&org.IsSystemicallyImportant, &org.MspStatus, &org.CreatedAt, &org.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		organizations = append(organizations, &org)
	}
	
	var totalCount int32
	err = dataService.db.QueryRowContext(ctx, countQuery, args[:len(args)-2]...).Scan(&totalCount)
	if err != nil {
		return nil, err
	}
	
	return &api.ListOrganizationsResponse{
		Organizations: organizations,
		TotalCount:    totalCount,
		Page:          req.Page,
		PageSize:      req.PageSize,
	}, nil
}

// CreateInvite - создание инвайт-кода
func (dataService *DataService) CreateInvite(ctx context.Context, req *api.CreateInviteRequest) (*api.InviteResponse, error) {
	code := generateInviteCode()
	expiresAt := time.Now().Add(time.Duration(req.ExpiresDays) * 24 * time.Hour)
	
	var inviteId int32
	err := dataService.db.QueryRowContext(ctx,
		`INSERT INTO invite_codes (code, email, organization_id, role_id, created_by, expires_at)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id`,
		code, req.Email, req.OrganizationId, req.RoleId, req.CreatedBy, expiresAt,
	).Scan(&inviteId)
	if err != nil {
		return nil, err
	}
	
	var invite api.Invite
	err = dataService.db.QueryRowContext(ctx,
		`SELECT id, code, email, organization_id, role_id, is_used, expires_at, created_at
		 FROM invite_codes WHERE id = $1`,
		inviteId,
	).Scan(&invite.Id, &invite.Code, &invite.Email, &invite.OrganizationId, 
		&invite.RoleId, &invite.IsUsed, &invite.ExpiresAt, &invite.CreatedAt)
	
	if err != nil {
		return nil, err
	}
	
	return &api.InviteResponse{Invite: &invite}, nil
}

// GetFinancialData - получение финансовых данных
func (dataService *DataService) GetFinancialData(ctx context.Context, req *api.GetFinancialDataRequest) (*api.FinancialDataResponse, error) {
	query := `SELECT id, year, revenue, net_profit, investments_moscow, export_volume
			  FROM financial_indicators 
			  WHERE organization_id = $1 AND destroyed = false`
	
	args := []interface{}{req.OrganizationId}
	
	if req.Year > 0 {
		query += " AND year = $2"
		args = append(args, req.Year)
	}
	
	query += " ORDER BY year DESC"
	
	rows, err := dataService.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var indicators []*api.FinancialIndicator
	for rows.Next() {
		var indicator api.FinancialIndicator
		err := rows.Scan(
			&indicator.Id, &indicator.Year, &indicator.Revenue, &indicator.NetProfit,
			&indicator.InvestmentsMoscow, &indicator.ExportVolume,
		)
		if err != nil {
			return nil, err
		}
		indicators = append(indicators, &indicator)
	}
	
	return &api.FinancialDataResponse{Indicators: indicators}, nil
}

// GetStaffData - получение данных о персонале
func (dataService *DataService) GetStaffData(ctx context.Context, req *api.GetStaffDataRequest) (*api.StaffDataResponse, error) {
	query := `SELECT id, year, total_staff, moscow_staff, total_payroll_fund, 
					 moscow_payroll_fund, avg_salary_total, avg_salary_moscow
			  FROM staff_indicators 
			  WHERE organization_id = $1 AND destroyed = false`
	
	args := []interface{}{req.OrganizationId}
	
	if req.Year > 0 {
		query += " AND year = $2"
		args = append(args, req.Year)
	}
	
	query += " ORDER BY year DESC"
	
	rows, err := dataService.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	var indicators []*api.StaffIndicator
	for rows.Next() {
		var indicator api.StaffIndicator
		err := rows.Scan(
			&indicator.Id, &indicator.Year, &indicator.TotalStaff, &indicator.MoscowStaff,
			&indicator.TotalPayrollFund, &indicator.MoscowPayrollFund,
			&indicator.AvgSalaryTotal, &indicator.AvgSalaryMoscow,
		)
		if err != nil {
			return nil, err
		}
		indicators = append(indicators, &indicator)
	}
	
	return &api.StaffDataResponse{Indicators: indicators}, nil
}

// Специализированные методы остаются без изменений
func (dataService *DataService) GetOrganization(ctx context.Context, getOrganizationRequest *api.GetOrganizationRequest) (*api.OrganizationResponse, error) {
	var organization api.Organization
	var query string
	var arguments []interface{}

	switch identifier := getOrganizationRequest.Identifier.(type) {
	case *api.GetOrganizationRequest_Id:
		query = `SELECT id, inn, name, full_name, spark_status, internal_status, final_status, 
						registration_date, added_to_registry_date, has_special_status, 
						is_systemically_important, msp_status, created_at, updated_at 
				 FROM active_organizations WHERE id = $1`
		arguments = []interface{}{identifier.Id}
	case *api.GetOrganizationRequest_Inn:
		query = `SELECT id, inn, name, full_name, spark_status, internal_status, final_status, 
						registration_date, added_to_registry_date, has_special_status, 
						is_systemically_important, msp_status, created_at, updated_at 
				 FROM active_organizations WHERE inn = $1`
		arguments = []interface{}{identifier.Inn}
	}

	err := dataService.db.QueryRowContext(ctx, query, arguments...).Scan(
		&organization.Id, &organization.Inn, &organization.Name, &organization.FullName, &organization.SparkStatus, &organization.InternalStatus,
		&organization.FinalStatus, &organization.RegistrationDate, &organization.AddedToRegistryDate, &organization.HasSpecialStatus,
		&organization.IsSystemicallyImportant, &organization.MspStatus, &organization.CreatedAt, &organization.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &api.OrganizationResponse{Organization: &organization}, nil
}

func (dataService *DataService) ValidateInvite(ctx context.Context, validateInviteRequest *api.ValidateInviteRequest) (*api.InviteResponse, error) {
	var invite api.Invite
	err := dataService.db.QueryRowContext(ctx,
		`SELECT id, code, email, organization_id, role_id, is_used, expires_at, created_at
		 FROM invite_codes WHERE code = $1 AND is_used = false AND expires_at > NOW()`,
		validateInviteRequest.Code,
	).Scan(&invite.Id, &invite.Code, &invite.Email, &invite.OrganizationId, 
		&invite.RoleId, &invite.IsUsed, &invite.ExpiresAt, &invite.CreatedAt)

	if err != nil {
		return nil, err
	}

	return &api.InviteResponse{Invite: &invite}, nil
}

func (dataService *DataService) UseInvite(ctx context.Context, useInviteRequest *api.UseInviteRequest) (*api.UserResponse, error) {
	transaction, err := dataService.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer transaction.Rollback()

	var invite api.Invite
	err = transaction.QueryRowContext(ctx,
		`UPDATE invite_codes SET is_used = true, used_at = NOW(), used_by = (SELECT id FROM users WHERE email = $1)
		 WHERE code = $2 AND is_used = false AND expires_at > NOW()
		 RETURNING id, code, email, organization_id, role_id, is_used, expires_at, created_at`,
		useInviteRequest.Email, useInviteRequest.Code,
	).Scan(&invite.Id, &invite.Code, &invite.Email, &invite.OrganizationId, 
		&invite.RoleId, &invite.IsUsed, &invite.ExpiresAt, &invite.CreatedAt)

	if err != nil {
		return nil, err
	}

	// Создаем пользователя
	_, err = dataService.CreateUser(ctx, &api.CreateUserRequest{
		Email:          useInviteRequest.Email,
		Password:       useInviteRequest.Password,
		FirstName:      useInviteRequest.FirstName,
		LastName:       useInviteRequest.LastName,
		Phone:          useInviteRequest.Phone,
		OrganizationId: invite.OrganizationId,
		RoleId:         invite.RoleId,
	})
	if err != nil {
		return nil, err
	}

	if err := transaction.Commit(); err != nil {
		return nil, err
	}

	// Возвращаем созданного пользователя
	return dataService.GetUser(ctx, &api.GetUserRequest{
		Identifier: &api.GetUserRequest_Email{Email: useInviteRequest.Email},
	})
}

func (dataService *DataService) SubmitForm(ctx context.Context, submitFormRequest *api.SubmitFormRequest) (*api.FormResponse, error) {
	var documentId int32
	err := dataService.db.QueryRowContext(ctx,
		`INSERT INTO document (pc_id, file_path_id, file_extension, form_id, user_id, status)
		 VALUES (1, 1, $1, $2, $3, 'submitted')
		 RETURNING id`,
		submitFormRequest.FileExtension, submitFormRequest.FormId, submitFormRequest.UserId,
	).Scan(&documentId)
	if err != nil {
		return nil, err
	}

	return &api.FormResponse{
		Id:        documentId,
		FormId:    submitFormRequest.FormId,
		UserId:    submitFormRequest.UserId,
		Status:    "submitted",
		FormData:  submitFormRequest.FormData,
		CreatedAt: nil,
	}, nil
}

// Вспомогательные методы (без изменений)
func (dataService *DataService) GetUser(ctx context.Context, getUserRequest *api.GetUserRequest) (*api.UserResponse, error) {
	var user api.User
	var query string
	var arguments []interface{}

	switch identifier := getUserRequest.Identifier.(type) {
	case *api.GetUserRequest_Id:
		query = `SELECT id, email, first_name, last_name, phone, organization_id, role_id, 
						is_active, is_verified, created_at, last_login, updated_at 
				 FROM active_users WHERE id = $1`
		arguments = []interface{}{identifier.Id}
	case *api.GetUserRequest_Email:
		query = `SELECT id, email, first_name, last_name, phone, organization_id, role_id, 
						is_active, is_verified, created_at, last_login, updated_at 
				 FROM active_users WHERE email = $1`
		arguments = []interface{}{identifier.Email}
	}

	err := dataService.db.QueryRowContext(ctx, query, arguments...).Scan(
		&user.Id, &user.Email, &user.FirstName, &user.LastName, &user.Phone,
		&user.OrganizationId, &user.RoleId, &user.IsActive, &user.IsVerified,
		&user.CreatedAt, &user.LastLogin, &user.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return &api.UserResponse{User: &user}, nil
}

func (dataService *DataService) CreateUser(ctx context.Context, createUserRequest *api.CreateUserRequest) (*api.UserResponse, error) {
	// Генерация солей и хешей пароля (упрощенная версия)
	saltEmail := generateSalt()
	saltPhone := generateSalt()
	passwordHashEmail := hashPassword(createUserRequest.Password + saltEmail)
	passwordHashPhone := hashPassword(createUserRequest.Password + saltPhone)

	var userId int32
	err := dataService.db.QueryRowContext(ctx,
		`INSERT INTO users (email, password_hash_email_sha256, password_hash_email_sha512256, 
		 password_hash_phone_sha256, password_hash_phone_sha512256, salt_email, salt_phone,
		 first_name, last_name, phone, organization_id, role_id, is_active, is_verified)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
		 RETURNING id`,
		createUserRequest.Email, 
		passwordHashEmail, passwordHashEmail, // Упрощенно - одинаковые хеши
		passwordHashPhone, passwordHashPhone,
		saltEmail, saltPhone,
		createUserRequest.FirstName, createUserRequest.LastName, createUserRequest.Phone, createUserRequest.OrganizationId, createUserRequest.RoleId,
		true, false,
	).Scan(&userId)
	if err != nil {
		return nil, err
	}

	// Возвращаем созданного пользователя
	return dataService.GetUser(ctx, &api.GetUserRequest{
		Identifier: &api.GetUserRequest_Id{Id: userId},
	})
}

// Вспомогательные функции (без изменений)
func generateSalt() string {
	return "salt_" + fmt.Sprintf("%d", time.Now().UnixNano())
}

func hashPassword(password string) string {
	return "hash_" + password
}

func generateInviteCode() string {
	return fmt.Sprintf("INV%d", time.Now().UnixNano())
}