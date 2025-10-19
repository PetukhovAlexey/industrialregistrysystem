package main

import (
    "database/sql"
    "fmt"
    "os"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    "time"
    "log"

    "github.com/xuri/excelize/v2"
    _ "github.com/lib/pq"
)

var db *sql.DB

type FileType int

const (
    FileTypeUnknown FileType = iota
    FileTypeExcelMultiYear
    FileTypeExcelSingleYear
    FileTypePDFSummary
    FileTypePDFDueDiligence
    FinancialReport
)

type BalanceData struct {
    TotalAssets          sql.NullFloat64
    Equity               sql.NullFloat64
    FixedAssets          sql.NullFloat64
    CurrentAssets        sql.NullFloat64
    LongTermLiabilities  sql.NullFloat64
    ShortTermLiabilities sql.NullFloat64
    Cash                 sql.NullFloat64
    AccountsReceivable   sql.NullFloat64
    Inventory            sql.NullFloat64
}

type IncomeData struct {
    Revenue    sql.NullFloat64
    NetProfit  sql.NullFloat64
}

func main() {
    var err error
    db, err = connectToDB()
    if err != nil {
        log.Fatal("Не удалось подключиться к БД:", err)
    }
    defer db.Close()

    // Проверяем соединение периодически
    go checkDBConnection()

    runServer()
}

func checkDBConnection() {
    ticker := time.NewTicker(5 * time.Minute)
    defer ticker.Stop()
    
    for range ticker.C {
        if err := db.Ping(); err != nil {
            log.Printf("Потеряно соединение с БД: %v", err)
            // Попытка переподключения
            if newDB, err := connectToDB(); err == nil {
                db.Close()
                db = newDB
                log.Println("Соединение с БД восстановлено")
            }
        }
    }
}

// fileExists проверяет существование файла
func fileExists(filePath string) bool {
    _, err := os.Stat(filePath)
    if os.IsNotExist(err) {
        return false
    }
    return err == nil
}

// readExcelFile читает данные из Excel файла
func readExcelFile(filePath string, originalFilename string, fileType FileType) ([]FinancialData, error) {
    if !fileExists(filePath) {
        return nil, fmt.Errorf("файл не существует: %s", filePath)
    }

    inn, ogrn, companyName := extractCompanyInfoFromFilename(filePath, originalFilename)
    
    f, err := excelize.OpenFile(filePath)
    if err != nil {
        return nil, err
    }
    defer f.Close()

    // Определяем годы
    years, err := detectYearsFromExcel(f, fileType)
    if err != nil {
        return nil, err
    }

    fmt.Printf("Обнаружены годы: %v\n", years)

    var allData []FinancialData

    // Читаем финансовые данные
    balanceData, err := readBalanceSheet(f, years)
    if err != nil {
        return nil, err
    }

    incomeData, err := readIncomeStatement(f, years)
    if err != nil {
        return nil, err
    }

    // Создаем записи для каждого года
    for _, year := range years {
        data := FinancialData{
            INN:      inn,
            OGRN:     ogrn,
            Name:     companyName,
            FullName: companyName,
            Year:     year,
            Date:     time.Now(),
            CreatedAt: time.Now(),
            UpdatedAt: time.Now(),
        }

        // Добавляем реквизиты в FullName только если они есть
        if inn != "" || ogrn != "" {
            data.FullName = fmt.Sprintf("%s (ИНН: %s, ОГРН: %s)", companyName, inn, ogrn)
        }

        // Добавляем данные из баланса
        if balance, exists := balanceData[year]; exists {
            data.TotalAssets = balance.TotalAssets
            data.Equity = balance.Equity
            data.FixedAssets = balance.FixedAssets
            data.CurrentAssets = balance.CurrentAssets
            data.LongTermLiabilities = balance.LongTermLiabilities
            data.ShortTermLiabilities = balance.ShortTermLiabilities
            data.Cash = balance.Cash
            data.AccountsReceivable = balance.AccountsReceivable
            data.Inventory = balance.Inventory
        }

        // Добавляем данные из отчета о прибылях и убытках
        if income, exists := incomeData[year]; exists {
            data.Revenue = income.Revenue
            data.NetProfit = income.NetProfit
        }

        // Заполняем обязательные поля
        fillRequiredFields(&data)
        
        allData = append(allData, data)
        
        fmt.Printf("Прочитаны данные за %s год: Выручка=%v, Прибыль=%v, Активы=%v\n", 
            year, data.Revenue, data.NetProfit, data.TotalAssets)
    }

    return allData, nil
}

// extractCompanyInfoFromFilename извлекает информацию о компании из имени файла
func extractCompanyInfoFromFilename(filename string, originalFilename string) (string, string, string) {
    baseName := filepath.Base(filename)
    baseName = strings.TrimSuffix(originalFilename, filepath.Ext(originalFilename))
    
    fmt.Printf("Анализ имени файла: %s\n", baseName)
    
    // Паттерн 1: ОГРН-годы
    if strings.Contains(baseName, "-202") || strings.Contains(baseName, "-20") {
        parts := strings.Split(baseName, "-")
        if len(parts) > 0 {
            ogrn := strings.TrimSpace(parts[0])
            if isNumeric(ogrn) && (len(ogrn) == 13 || len(ogrn) == 10 || len(ogrn) == 12) {
                inn := ""
                if len(ogrn) >= 10 {
                    inn = ogrn[len(ogrn)-10:]
                }
                companyName := fmt.Sprintf("Организация %s", ogrn)
                return inn, ogrn, companyName
            }
        }
    }
    
    var companyName string
    
    // Паттерн 2: Убираем "Бухгалтерская отчетность" и берем остальное как название
    prefixes := []string{
        "Бухгалтерская отчетность - ",
        "бухгалтерская отчетность - ",
        "Финансовая отчетность - ",
        "Бухгалтерская отчетность ",
        "бухгалтерская отчетность ",
        "Финансовая отчетность ",
    }
    
    for _, prefix := range prefixes {
        if strings.HasPrefix(baseName, prefix) {
            companyName = baseName[len(prefix):]
            companyName = removeTechnicalInfoFromEnd(companyName)
            break
        }
    }
    
    if companyName == "" {
        for _, prefix := range prefixes {
            if idx := strings.Index(baseName, prefix); idx != -1 {
                companyName = baseName[idx+len(prefix):]
                companyName = removeTechnicalInfoFromEnd(companyName)
                break
            }
        }
    }
    
    // Паттерн 3: Если не нашли через префиксы, используем все имя файла
    if companyName == "" {
        companyName = removeTechnicalInfoFromEnd(baseName)
    }
    
    // Паттерн 4: Извлечение ИНН и ОГРН из имени файла
    inn, ogrn := extractINNAndOGRN(baseName)
    if inn != "" || ogrn != "" {
        if companyName == "" {
            companyName = fmt.Sprintf("Организация %s", ogrn)
            if ogrn == "" && inn != "" {
                companyName = fmt.Sprintf("Организация %s", inn)
            }
        }
        return inn, ogrn, strings.TrimSpace(companyName)
    }
    
    // Паттерн 5: Поиск в базе данных
    companyName = strings.TrimSpace(companyName)
    if companyName != "" {
        dbInn, dbOgrn := findCompanyInDatabase(companyName)
        if dbInn != "" || dbOgrn != "" {
            fmt.Printf("Найдены реквизиты в БД для '%s': ИНН=%s, ОГРН=%s\n", companyName, dbInn, dbOgrn)
            return dbInn, dbOgrn, companyName
        }
    }
    
    // Паттерн 6: Если ничего не нашли
    if companyName == "" {
        companyName = "Неизвестная организация"
    } else if len(companyName) > 50 {
        companyName = companyName[:50] + "..."
    }
    
    fmt.Printf("Используем название: %s\n", companyName)
    return "", "", companyName
}

// findCompanyInDatabase ищет компанию в базе данных по названию
func findCompanyInDatabase(companyName string) (string, string) {
    if db == nil {
        fmt.Println("База данных не подключена")
        return "", ""
    }
    
    cleanName := strings.TrimSpace(companyName)
    
    queries := []string{
        "SELECT inn, ogrn FROM organisation WHERE name = $1 OR full_name = $1 LIMIT 1",
        "SELECT inn, ogrn FROM organisation WHERE name ILIKE $1 OR full_name ILIKE $1 LIMIT 1",
        "SELECT inn, ogrn FROM organisation WHERE name ILIKE $1 || '%' OR full_name ILIKE $1 || '%' LIMIT 1",
    }
    
    searchPatterns := []string{
        cleanName,
        "%" + cleanName + "%",
        cleanName,
    }
    
    for i, query := range queries {
        var inn, ogrn sql.NullString
        err := db.QueryRow(query, searchPatterns[i]).Scan(&inn, &ogrn)
        
        if err == nil {
            if inn.Valid || ogrn.Valid {
                innStr := ""
                ogrnStr := ""
                if inn.Valid {
                    innStr = inn.String
                }
                if ogrn.Valid {
                    ogrnStr = ogrn.String
                }
                return innStr, ogrnStr
            }
        } else if err != sql.ErrNoRows {
            fmt.Printf("Ошибка при поиске в БД: %v\n", err)
        }
    }
    
    // Дополнительный поиск для ООО
    if strings.HasPrefix(cleanName, "ООО ") {
        for i, query := range queries {
            var inn, ogrn sql.NullString
            err := db.QueryRow(query, searchPatterns[i]).Scan(&inn, &ogrn)
            
            if err == nil {
                if inn.Valid || ogrn.Valid {
                    innStr := ""
                    ogrnStr := ""
                    if inn.Valid {
                        innStr = inn.String
                    }
                    if ogrn.Valid {
                        ogrnStr = ogrn.String
                    }
                    return innStr, ogrnStr
                }
            }
        }
    }
    
    return "", ""
}

// extractINNAndOGRN пытается найти ИНН и ОГРН в строке
func extractINNAndOGRN(text string) (string, string) {
    inn := ""
    ogrn := ""
    
    words := strings.Fields(text)
    for _, word := range words {
        cleanWord := strings.Trim(word, " ,.-_()")
        if isNumeric(cleanWord) {
            switch len(cleanWord) {
            case 10: // ИНН юридического лица
                inn = cleanWord
            case 12: // ИНН физического лица
                inn = cleanWord
            case 13: // ОГРН
                ogrn = cleanWord
            }
        }
    }
    
    return inn, ogrn
}

// removeTechnicalInfoFromEnd удаляет техническую информацию только с конца названия
func removeTechnicalInfoFromEnd(name string) string {
    originalName := name
    
    // Удаляем расширения файлов
    name = strings.TrimSuffix(name, ".xlsx")
    name = strings.TrimSuffix(name, ".xls")
    name = strings.TrimSuffix(name, ".pdf")
    name = strings.TrimSuffix(name, ".PDF")
    
    // Удаляем годы в конце
    re := regexp.MustCompile(`[\s\-_]*((19|20)\d{2})[\s\-_]*$`)
    name = re.ReplaceAllString(name, "")
    
    // Удаляем технические суффиксы
    technicalSuffixes := []string{
        "отчетность", "отчет", "баланс", 
        "финансовый", "бухгалтерский",
        "за год", "год", "г.", 
        "копия", "сканирование", "scan",
        "final", "draft", "ver", "version",
    }
    
    for _, suffix := range technicalSuffixes {
        patterns := []string{
            " " + suffix + "$",
            " - " + suffix + "$", 
            "_" + suffix + "$",
            "-" + suffix + "$",
        }
        for _, pattern := range patterns {
            name = strings.TrimSuffix(name, pattern)
        }
    }
    
    name = strings.Trim(name, " -_")
    
    if name == "" {
        return originalName
    }
    
    return strings.TrimSpace(name)
}

// isNumeric проверяет, состоит ли строка только из цифр
func isNumeric(s string) bool {
    if s == "" {
        return false
    }
    for _, char := range s {
        if char < '0' || char > '9' {
            return false
        }
    }
    return true
}

// detectYearsFromExcel определяет годы из Excel файла
func detectYearsFromExcel(f *excelize.File, fileType FileType) ([]string, error) {
    rows, err := f.GetRows("Бухгалтерский баланс")
    if err != nil {
        return nil, err
    }

    if len(rows) == 0 {
        return nil, fmt.Errorf("файл не содержит данных")
    }

    if fileType == FileTypeExcelMultiYear {
        headerRow := rows[0]
        var years []string
        
        for _, cell := range headerRow {
            if isYear(cell) {
                years = append(years, strings.TrimSpace(cell))
            }
        }
        
        if len(years) > 0 {
            return years, nil
        }
    }

    singleYear := extractYearFromSingleFormat(rows)
    if singleYear != "" {
        return []string{singleYear}, nil
    }

    currentYear := fmt.Sprintf("%d", time.Now().Year())
    return []string{currentYear}, nil
}

func isYear(value string) bool {
    value = strings.TrimSpace(value)
    if len(value) != 4 {
        return false
    }
    year, err := strconv.Atoi(value)
    if err != nil {
        return false
    }
    return year >= 2000 && year <= 2100
}

func extractYearFromSingleFormat(rows [][]string) string {
    for _, row := range rows {
        for _, cell := range row {
            if isYear(cell) {
                return strings.TrimSpace(cell)
            }
        }
    }
    return ""
}

// readBalanceSheet читает данные из баланса
func readBalanceSheet(f *excelize.File, years []string) (map[string]BalanceData, error) {
    rows, err := f.GetRows("Бухгалтерский баланс")
    if err != nil {
        return nil, err
    }

    balanceMap := make(map[string]map[string]string)
    
    for _, row := range rows {
        if len(row) < 3 {
            continue
        }
        
        code := strings.TrimSpace(row[1])
        if code == "" {
            continue
        }
        
        balanceMap[code] = make(map[string]string)
        
        // Для формата с одним годом данные в колонке C (индекс 2)
        for i, year := range years {
            if i == 0 && len(row) > 2 {
                balanceMap[code][year] = strings.TrimSpace(row[2])
            }
        }
    }

    result := make(map[string]BalanceData)
    
    for _, year := range years {
        result[year] = BalanceData{
            TotalAssets:          parseNumericValue(balanceMap["1600"][year]),
            Equity:               parseNumericValue(balanceMap["1300"][year]),
            FixedAssets:          parseNumericValue(balanceMap["1150"][year]),
            CurrentAssets:        parseNumericValue(balanceMap["1200"][year]),
            LongTermLiabilities:  parseNumericValue(balanceMap["1400"][year]),
            ShortTermLiabilities: parseNumericValue(balanceMap["1500"][year]),
            Cash:                 parseNumericValue(balanceMap["1250"][year]),
            AccountsReceivable:   parseNumericValue(balanceMap["1230"][year]),
            Inventory:            parseNumericValue(balanceMap["1210"][year]),
        }
    }

    return result, nil
}

// readIncomeStatement читает данные из отчета о прибылях и убытках
func readIncomeStatement(f *excelize.File, years []string) (map[string]IncomeData, error) {
    rows, err := f.GetRows("Отчет о фин. результатах")
    if err != nil {
        return nil, err
    }

    incomeMap := make(map[string]map[string]string)
    
    for _, row := range rows {
        if len(row) < 3 {
            continue
        }
        
        code := strings.TrimSpace(row[1])
        if code == "" {
            continue
        }
        
        incomeMap[code] = make(map[string]string)
        
        // Для формата с одним годом данные в колонке C (индекс 2)
        for i, year := range years {
            if i == 0 && len(row) > 2 {
                incomeMap[code][year] = strings.TrimSpace(row[2])
            }
        }
    }

    result := make(map[string]IncomeData)
    
    for _, year := range years {
        result[year] = IncomeData{
            Revenue:   parseNumericValue(incomeMap["2110"][year]),
            NetProfit: parseNumericValue(incomeMap["2400"][year]),
        }
    }

    return result, nil
}

// parseNumericValue парсит числовые значения
func parseNumericValue(value string) sql.NullFloat64 {
    if value == "" || value == "-" {
        return sql.NullFloat64{Valid: false}
    }
    
    cleaned := strings.ReplaceAll(strings.TrimSpace(value), ",", ".")
    cleaned = strings.ReplaceAll(cleaned, " ", "")
    
    if cleaned == "0" {
        return sql.NullFloat64{Float64: 0, Valid: true}
    }
    
    floatVal, err := strconv.ParseFloat(cleaned, 64)
    if err != nil {
        fmt.Printf("Ошибка парсинга значения '%s': %v\n", value, err)
        return sql.NullFloat64{Valid: false}
    }
    
    return sql.NullFloat64{Float64: floatVal, Valid: true}
}
