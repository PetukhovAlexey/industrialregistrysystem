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
)

// DueDiligenceReportParser парсер для отчетов о должной осмотрительности
type DueDiligenceReportParser struct{}

// ParseDueDiligenceReport парсит отчет о должной осмотрительности
func ParseDueDiligenceReport(content string) ([]FinancialData, error) {
	var financialData []FinancialData

	// Извлекаем основную информацию о компании
	companyName := extractCompanyNameFromDueDiligence(content)
	inn := extractINNFromDueDiligence(content)
	ogrn := extractOGRNFromDueDiligence(content)
	
	// Извлекаем дополнительные данные
	registrationDate := extractRegistrationDateFromDueDiligence(content)
	legalAddress := extractLegalAddressFromDueDiligence(content)
	staffCount := extractStaffCount(content)
	mainOKVED := extractMainOKVED(content)

	// Извлекаем финансовые показатели если они есть
	revenue, profit := extractFinancialIndicators(content)
	
	// Создаем запись с текущим годом
	if inn != "" || ogrn != "" {
		currentYear := strconv.Itoa(time.Now().Year())
		
		data := FinancialData{
			Year:             currentYear,
			INN:              inn,
			OGRN:             ogrn,
			Name:             companyName,
			FullName:         companyName,
			Revenue:          revenue,
			NetProfit:        profit,
			RegistrationDate: registrationDate,
			LegalAddress:     legalAddress,
			StaffCount:       staffCount,
			MainOKVED:        mainOKVED,
			Date:             time.Now(),
		}
		
		financialData = append(financialData, data)
	}

	return financialData, nil
}

func extractCompanyNameFromDueDiligence(content string) string {
	// Поиск полного наименования
	patterns := []string{
		`Полное наименование\s*\n([^\n]+)`,
		`ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ "[^"]+"`,
		`ООО "[^"]+"`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		} else if len(matches) > 0 {
			return strings.TrimSpace(matches[0])
		}
	}
	
	// Поиск по заголовку
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		if strings.Contains(line, "ООО \"") && len(line) < 100 {
			return strings.TrimSpace(line)
		}
	}
	
	return "Неизвестная компания"
}

func extractINNFromDueDiligence(content string) string {
	// Поиск ИНН в реквизитах
	patterns := []string{
		`ИНН\s*(\d{10,12})`,
		`ИНН/КПП\s*(\d{10})/\d{9}`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	
	return ""
}

func extractOGRNFromDueDiligence(content string) string {
	// Поиск ОГРН в реквизитах
	patterns := []string{
		`ОГРН\s*(\d{13})`,
		`\b\d{13}\b`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			return matches[1]
		}
	}
	
	return ""
}

func extractRegistrationDateFromDueDiligence(content string) string {
	// Извлечение даты регистрации
	re := regexp.MustCompile(`Дата регистрации\s*(\d{1,2}\s+\w+\s+\d{4}\s*г\.?)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func extractLegalAddressFromDueDiligence(content string) string {
	// Извлечение юридического адреса
	re := regexp.MustCompile(`Юридический адрес\s*([^\n]+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func extractStaffCount(content string) sql.NullInt64 {
	// Извлечение среднесписочной численности
	re := regexp.MustCompile(`Среднесписочная численность\s*(\d+)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		count, err := strconv.Atoi(matches[1])
		if err == nil {
			return sql.NullInt64{Int64: int64(count), Valid: true}
		}
	}
	return sql.NullInt64{Valid: false}
}

func extractMainOKVED(content string) string {
	// Извлечение основного вида деятельности
	re := regexp.MustCompile(`Основной вид деятельности\s*(\d{2}\.\d{2}[^\n]*)`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

func extractFinancialIndicators(content string) (sql.NullFloat64, sql.NullFloat64) {
	var revenue, profit sql.NullFloat64
	
	// Поиск финансовых показателей в тексте
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		// Поиск выручки
		if strings.Contains(line, "Выручка") || strings.Contains(line, "выручка") {
			revenue = extractNumericValueFromText(line)
		}
		// Поиск прибыли
		if strings.Contains(line, "Прибыль") || strings.Contains(line, "прибыль") {
			profit = extractNumericValueFromText(line)
		}
	}
	
	return revenue, profit
}

func extractNumericValueFromText(line string) sql.NullFloat64 {
	// Поиск числовых значений в тексте (в млрд, млн, тыс. руб.)
	patterns := []string{
		`(\d+[,\d]*)\s*млрд`,
		`(\d+[,\d]*)\s*млн`,
		`(\d+[,\d]*)\s*тыс`,
		`(\d+[,\d]*)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			// Очистка и преобразование числа
			cleaned := strings.ReplaceAll(matches[1], ",", ".")
			cleaned = strings.ReplaceAll(cleaned, " ", "")
			
			value, err := strconv.ParseFloat(cleaned, 64)
			if err == nil {
				// Применение множителя в зависимости от единицы измерения
				multiplier := 1.0
				if strings.Contains(line, "млрд") {
					multiplier = 1e9
				} else if strings.Contains(line, "млн") {
					multiplier = 1e6
				} else if strings.Contains(line, "тыс") {
					multiplier = 1e3
				}
				
				return sql.NullFloat64{Float64: value * multiplier, Valid: true}
			}
		}
	}
	
	return sql.NullFloat64{Valid: false}
}

// extractFinancialDataFromSummaryPDFWrapper обертка для вызова функции из SummaryReportPdf.go
func extractFinancialDataFromSummaryPDFWrapper(content string) ([]FinancialData, error) {
	lines := strings.Split(content, "\n")
	companyName := extractCompanyNameFromDueDiligence(content)
	inn := extractINNFromDueDiligence(content)
	ogrn := extractOGRNFromDueDiligence(content)
	registrationDate := extractRegistrationDateFromDueDiligence(content)
	legalAddress := extractLegalAddressFromDueDiligence(content)
	director := extractDirectorFromDueDiligence(content)
	authorizedCapital := extractAuthorizedCapitalFromDueDiligence(content)

	yearData := extractFinancialDataByPattern(lines, companyName, inn, ogrn, 
		registrationDate, legalAddress, director, authorizedCapital)

	// Конвертируем map в slice
	var financialData []FinancialData
	for _, data := range yearData {
		financialData = append(financialData, data)
	}

	return financialData, nil
}

func extractDirectorFromDueDiligence(content string) string {
	patterns := []string{
		`Генеральный директор\s*([^\n]+)`,
		`Директор\s*([^\n]+)`,
		`Руководитель\s*([^\n]+)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			return strings.TrimSpace(matches[1])
		}
	}
	return ""
}

func extractAuthorizedCapitalFromDueDiligence(content string) float64 {
	patterns := []string{
		`Уставный капитал\s*([\d\s,\.]+)`,
		`Уставный капитал[^\d]*([\d\s,\.]+)`,
	}
	
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			// Очищаем строку от пробелов и заменяем запятые на точки
			cleanStr := strings.ReplaceAll(strings.ReplaceAll(matches[1], " ", ""), ",", ".")
			if value, err := strconv.ParseFloat(cleanStr, 64); err == nil {
				return value
			}
		}
	}
	return 0
}

// Обновленная функция readPDFFile с поддержкой разных типов PDF
func readPDFFile(filePath string, fileType FileType) ([]FinancialData, error) {
	// Читаем файл
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("ошибка открытия файла: %v", err)
	}
	defer file.Close()

	// Получаем размер файла
	fileInfo, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("ошибка получения информации о файле: %v", err)
	}

	// Парсим PDF
	content, err := parsePDFContent(file, fileInfo.Size())
	if err != nil {
		return nil, fmt.Errorf("ошибка парсинга PDF: %v", err)
	}

	// Выбираем парсер в зависимости от типа файла
	switch fileType {
	case FileTypePDFSummary:
		return extractFinancialDataFromSummaryPDF(content)
	case FileTypePDFDueDiligence:
		return ParseDueDiligenceReport(content)
	case FinancialReport:
		return ParseEgrlPDFContent(content)
	default:
		return nil, fmt.Errorf("неподдерживаемый тип файла: %v", fileType)
	}
}

// Функция для определения типа PDF файла по содержимому
func detectPDFType(content string) FileType {
	if strings.Contains(content, "ЕГРЮЛ") || 
       strings.Contains(content, "Единый государственный реестр юридических лиц") ||
       strings.Contains(content, "Сведения о юридическом лице") ||
       strings.Contains(content, "Выписка из ЕГРЮЛ") ||
       strings.Contains(content, "выписка из ЕГРЮЛ") {
        return FinancialReport // или создайте отдельный тип FileTypeEGRUL если нужно
    }
	
	// Проверяем признаки отчета о должной осмотрительности
	if strings.Contains(content, "должной осмотрительности") || 
	   strings.Contains(content, "Анализ надежности") ||
	   strings.Contains(content, "Риски неисполнения обязательств") ||
	   strings.Contains(content, "Должная осмотрительность") {
		return FileTypePDFDueDiligence
	}
	
	// Проверяем признаки сводного отчета с финансовыми данными по годам
	if (strings.Contains(content, "Выручка:") || strings.Contains(content, "Прибыль:")) && 
	   containsYear(content) {
		return FileTypePDFSummary
	}
	
	// Проверяем наличие финансовой таблицы с годами
	lines := strings.Split(content, "\n")
	yearCount := 0
	for _, line := range lines {
		if containsYear(line) {
			yearCount++
		}
	}
	
	if yearCount >= 2 {
		return FileTypePDFSummary
	}
	
	// По умолчанию считаем отчетом о должной осмотрительности
	return FileTypePDFDueDiligence
}

// Улучшенная функция readFinancialFile с автоопределением типа PDF
func readFinancialFile(filePath string, originalFilename string, fileType FileType) ([]FinancialData, error) {
	if !fileExists(filePath) {
		return nil, fmt.Errorf("файл не существует: %s", filePath)
	}

	// Для PDF файлов определяем тип автоматически если не указан явно
	if fileType == FileTypeUnknown && strings.ToLower(filepath.Ext(filePath)) == ".pdf" {
		file, err := os.Open(filePath)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		
		fileInfo, err := file.Stat()
		if err != nil {
			return nil, err
		}
		
		content, err := parsePDFContent(file, fileInfo.Size())
		if err != nil {
			return nil, err
		}
		
		fileType = detectPDFType(content)
		fmt.Printf("Автоопределен тип PDF: %v\n", fileType)
	}

	switch fileType {
	case FileTypeExcelMultiYear, FileTypeExcelSingleYear:
		return readExcelFile(filePath, originalFilename, fileType)
	case FileTypePDFSummary, FileTypePDFDueDiligence, FinancialReport:
		return readPDFFile(filePath, fileType)
	default:
		return nil, fmt.Errorf("неподдерживаемый тип файла: %v", fileType)
	}
}

// containsYear проверяет, содержит ли строка год в формате 20xx
func containsYear(line string) bool {
	yearPattern := `\b20\d{2}\b`
	matched, _ := regexp.MatchString(yearPattern, line)
	return matched
}