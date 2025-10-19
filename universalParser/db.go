package main

import (
    "database/sql"
    "fmt"
    "os"
    "strings"
    "time"

    _ "github.com/lib/pq"
)

type FinancialData struct {
    // Основные идентификаторы
    ID                   int64
    DocID                int64
    DocType              int64
    INN                  string
    OGRN                 string
    Name                 string
    Year                 string
    
    // Основные реквизиты
    FullName             string
    RegistrationDate     string
    LegalAddress         string
    Director             string
    AuthorizedCapital    float64
    
    // Финансовые показатели
    Cost                 float64
    Profit               float64
    Revenue              sql.NullFloat64
    NetProfit            sql.NullFloat64
    TotalAssets          sql.NullFloat64
    Equity               sql.NullFloat64
    FixedAssets          sql.NullFloat64
    CurrentAssets        sql.NullFloat64
    LongTermLiabilities  sql.NullFloat64
    ShortTermLiabilities sql.NullFloat64
    Cash                 sql.NullFloat64
    AccountsReceivable   sql.NullFloat64
    Inventory            sql.NullFloat64
    
    // Все остальные поля из таблицы...
    AddedToRegistryDate         sql.NullTime
    ProductionAddress           sql.NullString
    AdditionalSiteAddress       sql.NullString
    MainIndustry                sql.NullString
    MainSubindustry             sql.NullString
    AdditionalIndustry          sql.NullString
    AdditionalSubindustry       sql.NullString
    IndustryPresentations       sql.NullString
    MainOKVEDCode               sql.NullString
    MainOKVEDActivity           sql.NullString
    ProductionOKVEDCode         sql.NullString
    ProductionOKVEDActivity     sql.NullString
    CompanyInfo                 sql.NullString
    CompanySizeCategory         sql.NullString
    CompanySizeByStaff          sql.NullString
    CompanySizeByRevenue        sql.NullString
    LeaderName                  sql.NullString
    HeadOrganization            sql.NullString
    HeadOrganizationINN         sql.NullString
    HeadOrganizationRelationType sql.NullString
    LeaderContacts              sql.NullString
    LeaderEmail                 sql.NullString
    EmployeeContact             sql.NullString
    PhoneNumber                 sql.NullString
    EmergencyContact            sql.NullString
    Website                     sql.NullString
    GeneralEmail                sql.NullString
    SupportMeasuresInfo         sql.NullString
    HasSpecialStatus            sql.NullBool
    SummarySite                 sql.NullString
    GotMoscowSupport            sql.NullBool
    IsSystemicallyImportant     sql.NullBool
    MSPStatus                   sql.NullString
    IsTheOne                    sql.NullBool
    HasStateOrder               sql.NullBool
    ProductionCapacityUtilization sql.NullFloat64
    HasExportSupplies            sql.NullBool
    ExportVolumePreviousYear     sql.NullFloat64
    ExportCountriesList          sql.NullString
    TNVEDCode                    sql.NullString
    IndustryBySparkAndRef       sql.NullString
    District                     sql.NullString
    Area                         sql.NullString
    TotalStaff                   sql.NullInt64
    MoscowStaff                  sql.NullInt64
    TotalPayroll                 sql.NullFloat64
    MoscowPayroll                sql.NullFloat64
    AvgSalaryTotal               sql.NullFloat64
    AvgSalaryMoscow              sql.NullFloat64
    TotalTaxes                   sql.NullFloat64
    ProfitTax                    sql.NullFloat64
    PropertyTax                  sql.NullFloat64
    LandTax                      sql.NullFloat64
    PersonalIncomeTax            sql.NullFloat64
    TransportTax                 sql.NullFloat64
    OtherTaxes                   sql.NullFloat64
    ExciseTax                    sql.NullFloat64
    InvestmentsMoscow            sql.NullFloat64
    ExportVolume                 sql.NullFloat64
    LandCadastralNumber          sql.NullString
    LandArea                     sql.NullFloat64
    LandPermittedUse             sql.NullString
    LandOwnershipType            sql.NullString
    LandOwner                    sql.NullString
    BuildingCadastralNumber      sql.NullString
    BuildingArea                 sql.NullFloat64
    BuildingPermittedUse         sql.NullString
    BuildingTypeAndPurpose       sql.NullString
    BuildingOwnershipType        sql.NullString
    BuildingOwner                sql.NullString
    ProductionArea               sql.NullFloat64
    ProductName                  sql.NullString
    StandardizedProductName      sql.NullString
    ProducedProductsList         sql.NullString
    ProductsByTypeAndSegment     sql.NullString
    ProductCatalog               sql.NullString
    LegalAddressLatitude         sql.NullFloat64
    LegalAddressLongitude        sql.NullFloat64
    ProductionAddressLatitude    sql.NullFloat64
    ProductionAddressLongitude   sql.NullFloat64
    AdditionalSiteLatitude       sql.NullFloat64
    AdditionalSiteLongitude      sql.NullFloat64
    
    // Системные поля
    Date                 time.Time
    CreatedAt            time.Time
    UpdatedAt            time.Time
    StartPeriod          time.Time
    FinishPeriod         time.Time
    Revision             int64
    Destroyed            bool
    
    // Дополнительные поля из PDF
    StaffCount                   sql.NullInt64
    MainOKVED                    string
    OKVEDDescription             string
    Status                       string
    KPP                          string
    TaxAuthority                 string
    LegalForm                    string
}

// Document представляет запись в таблице documents
type Document struct {
	ID               int64
	FilePath         string
	FileName         string
	FileExtension    string
	OriginalFileName string
	FileSize         int64
	DocumentTypeID   int64
	Organisation     sql.NullInt64
	CreatedAt        time.Time
}

// SaveDocument сохраняет документ в БД
func SaveDocument(db *sql.DB, doc *Document) (int64, error) {
	query := `
	INSERT INTO documents (
		file_path, file_name, file_extension, original_file_name, 
		file_size, document_type_id, created_at
	) VALUES ($1, $2, $3, $4, $5, $6, $7)
	RETURNING id`

	var id int64
	err := db.QueryRow(query,
		doc.FilePath,
		doc.FileName,
		doc.FileExtension,
		doc.OriginalFileName,
		doc.FileSize,
		doc.DocumentTypeID,
		doc.CreatedAt,
	).Scan(&id)

	if err != nil {
		return 0, fmt.Errorf("ошибка сохранения документа в БД: %v", err)
	}

	return id, nil
}

func readFileToString(filename string) (string, error) {
    content, err := os.ReadFile(filename)
    if err != nil {
        return "", err
    }
    return string(content), nil
}

func connectToDB() (*sql.DB, error) {
    connStr, err := readFileToString("db.conf")
    if err != nil {
        return nil, err
    }
    db, err := sql.Open("postgres", connStr)
    if err != nil {
        return nil, err
    }
    
    err = db.Ping()
    if err != nil {
        return nil, err
    }
    
    return db, nil
}

// InsertFinancialData вставляет все данные финансовой карточки в БД
func InsertFinancialData(db *sql.DB, data []FinancialData) (int, error) {
    inserted := 0

    for _, record := range data {
        query := `
        INSERT INTO indecaters (
            document, document_type, inn, ogrn, name, full_name, registration_date,
            added_to_registry_date, legal_address, production_address, additional_site_address,
            main_industry, main_subindustry, additional_industry, additional_subindustry,
            industry_presentations, main_okved_code, main_okved_activity,
            production_okved_code, production_okved_activity, company_info,
            company_size_category, company_size_by_staff, company_size_by_revenue,
            leader_name, head_organization, head_organization_inn, head_organization_relation_type,
            leader_contacts, leader_email, employee_contact, phone_number, emergency_contact,
            website, general_email, support_measures_info, has_special_status, summary_site,
            got_moscow_support, is_systemically_important, msp_status, is_the_one,
            has_state_order, production_capacity_utilization, has_export_supplies,
            export_volume_previous_year, export_countries_list, tn_ved_code,
            industry_by_spark_and_ref, district, area, revenue, net_profit,
            total_staff, moscow_staff, total_payroll, moscow_payroll,
            avg_salary_total, avg_salary_moscow, total_taxes, profit_tax,
            property_tax, land_tax, personal_income_tax, transport_tax,
            other_taxes, excise_tax, investments_moscow, export_volume,
            land_cadastral_number, land_area, land_permitted_use, land_ownership_type,
            land_owner, building_cadastral_number, building_area, building_permitted_use,
            building_type_and_purpose, building_ownership_type, building_owner,
            production_area, product_name, standardized_product_name, produced_products_list,
            products_by_type_and_segment, product_catalog, legal_address_latitude,
            legal_address_longitude, production_address_latitude, production_address_longitude,
            additional_site_latitude, additional_site_longitude, start_period, finish_period,
            revision, destroyed, created_at, updated_at, total_assets, equity,
            fixed_assets, current_assets, long_term_liabilities, short_term_liabilities,
            cash, accounts_receivable, inventory
        ) VALUES (
            $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18,
            $19, $20, $21, $22, $23, $24, $25, $26, $27, $28, $29, $30, $31, $32, $33, $34,
            $35, $36, $37, $38, $39, $40, $41, $42, $43, $44, $45, $46, $47, $48, $49, $50,
            $51, $52, $53, $54, $55, $56, $57, $58, $59, $60, $61, $62, $63, $64, $65, $66,
            $67, $68, $69, $70, $71, $72, $73, $74, $75, $76, $77, $78, $79, $80, $81, $82,
            $83, $84, $85, $86, $87, $88, $89, $90, $91, $92, $93, $94, $95, $96, $97, $98,
            $99, $100, $101, $102, $103, $104, $105, $106, $107
        )`

        startPeriod, finishPeriod := createPeriodFromYear(record.Year)
        now := time.Now()
        
        _, err := db.Exec(query,
            // Связь с документом и основные идентификаторы (2 поля)
            record.DocID, record.DocType, 
            
            // Основные реквизиты (5 полей)
            toNullString(record.INN), toNullString(record.OGRN), 
            toNullString(record.Name), toNullString(record.FullName),
            toNullDate(record.RegistrationDate),
            
            // Даты и адреса (4 поля)
            record.AddedToRegistryDate,
            toNullString(record.LegalAddress), record.ProductionAddress, record.AdditionalSiteAddress,
            
            // Отраслевая информация (10 полей)
            record.MainIndustry, record.MainSubindustry, record.AdditionalIndustry,
            record.AdditionalSubindustry, record.IndustryPresentations,
            record.MainOKVEDCode, record.MainOKVEDActivity, record.ProductionOKVEDCode,
            record.ProductionOKVEDActivity, record.CompanyInfo,
            
            // Размер компании (3 поля)
            record.CompanySizeCategory, record.CompanySizeByStaff, record.CompanySizeByRevenue,
            
            // Руководство и контакты (11 полей)
            record.LeaderName, record.HeadOrganization, record.HeadOrganizationINN,
            record.HeadOrganizationRelationType, record.LeaderContacts, record.LeaderEmail,
            record.EmployeeContact, record.PhoneNumber, record.EmergencyContact,
            record.Website, record.GeneralEmail,
            
            // Поддержка и статусы (8 полей)
            record.SupportMeasuresInfo, record.HasSpecialStatus, record.SummarySite,
            record.GotMoscowSupport, record.IsSystemicallyImportant, record.MSPStatus,
            record.IsTheOne, record.HasStateOrder,
            
            // Производственные показатели (5 полей)
            record.ProductionCapacityUtilization, record.HasExportSupplies,
            record.ExportVolumePreviousYear, record.ExportCountriesList, record.TNVEDCode,
            
            // Коды и классификация (3 поля)
            record.IndustryBySparkAndRef, record.District, record.Area,
            
            // Основные финансовые показатели (2 поля)
            record.Revenue, record.NetProfit,
            
            // Персонал (6 полей)
            record.TotalStaff, record.MoscowStaff, record.TotalPayroll, record.MoscowPayroll,
            record.AvgSalaryTotal, record.AvgSalaryMoscow,
            
            // Налоги (8 полей)
            record.TotalTaxes, record.ProfitTax, record.PropertyTax, record.LandTax,
            record.PersonalIncomeTax, record.TransportTax, record.OtherTaxes, record.ExciseTax,
            
            // Инвестиции и экспорт (2 поля)
            record.InvestmentsMoscow, record.ExportVolume,
            
            // Земельные участки (6 полей)
            record.LandCadastralNumber, record.LandArea, record.LandPermittedUse,
            record.LandOwnershipType, record.LandOwner, record.BuildingCadastralNumber,
            
            // Здания и сооружения (6 полей)
            record.BuildingArea, record.BuildingPermittedUse, record.BuildingTypeAndPurpose,
            record.BuildingOwnershipType, record.BuildingOwner, record.ProductionArea,
            
            // Производство и продукция (6 полей)
            record.ProductName, record.StandardizedProductName, record.ProducedProductsList,
            record.ProductsByTypeAndSegment, record.ProductCatalog, record.LegalAddressLatitude,
            
            // Координаты (5 полей)
            record.LegalAddressLongitude, record.ProductionAddressLatitude, 
            record.ProductionAddressLongitude, record.AdditionalSiteLatitude, 
            record.AdditionalSiteLongitude,
            
            // Периоды и системные поля (6 полей)
            startPeriod, finishPeriod, record.Revision, record.Destroyed, now, now,
            
            // Балансовые показатели (9 полей)
            record.TotalAssets, record.Equity, record.FixedAssets, record.CurrentAssets,
            record.LongTermLiabilities, record.ShortTermLiabilities, record.Cash,
            record.AccountsReceivable, record.Inventory,
        )

        if err != nil {
            if strings.Contains(err.Error(), "duplicate key") {
                fmt.Printf("Запись с ИНН %s (ОГРН %s) за %s год уже существует\n", record.INN, record.OGRN, record.Year)
                continue
            }
            return inserted, fmt.Errorf("ошибка вставки данных для ИНН %s: %v", record.INN, err)
        }

        inserted++
        fmt.Printf("Добавлена полная запись для ИНН %s (ОГРН %s) за %s год\n", record.INN, record.OGRN, record.Year)
    }

    return inserted, nil
}

// Вспомогательные функции для преобразования типов
func toNullString(s string) sql.NullString {
    if s == "" {
        return sql.NullString{Valid: false}
    }
    return sql.NullString{String: s, Valid: true}
}

func toNullDate(dateStr string) sql.NullTime {
    if dateStr == "" {
        return sql.NullTime{Valid: false}
    }
    
    formats := []string{
        "02.01.2006",
        "02/01/2006", 
        "2006-01-02",
        "02.01.06",
    }
    
    for _, format := range formats {
        if t, err := time.Parse(format, dateStr); err == nil {
            return sql.NullTime{Time: t, Valid: true}
        }
    }
    
    return sql.NullTime{Valid: false}
}

func toNullFloat64(f float64) sql.NullFloat64 {
    if f == 0 {
        return sql.NullFloat64{Valid: false}
    }
    return sql.NullFloat64{Float64: f, Valid: true}
}

// createPeriodFromYear создает периоды начала и конца года на основе строки года
func createPeriodFromYear(year string) (time.Time, time.Time) {
    start, _ := time.Parse("2006-01-02", year+"-01-01")
    end, _ := time.Parse("2006-01-02", year+"-12-31")
    return start, end
}

// determineCompanySize определяет категорию компании по выручке
func determineCompanySize(revenue sql.NullFloat64) string {
    if !revenue.Valid {
        return "неизвестно"
    }
    
    rev := revenue.Float64
    switch {
    case rev < 120000000:
        return "микропредприятие"
    case rev < 800000000:
        return "малое предприятие"
    case rev < 2000000000:
        return "среднее предприятие"
    default:
        return "крупное предприятие"
    }
}