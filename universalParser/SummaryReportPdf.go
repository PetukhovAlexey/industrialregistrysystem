package main

import (
	"database/sql"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ledongthuc/pdf"
)

func parsePDFContent(file *os.File, fileSize int64) (string, error) {
	// Используем библиотеку для парсинга PDF
	reader, err := pdf.NewReader(file, fileSize)
	if err != nil {
		return "", err
	}

	var content strings.Builder
	totalPage := reader.NumPage()
	for pageIndex := 1; pageIndex <= totalPage; pageIndex++ {
		page := reader.Page(pageIndex)
		if page.V.IsNull() {
			continue
		}
		text, err := page.GetPlainText(nil)
		if err != nil {
			continue
		}
		content.WriteString(text)
	}

	return content.String(), nil
}

// extractFinancialDataFromSummaryPDF извлекает финансовые данные из сводного PDF отчета
func extractFinancialDataFromSummaryPDF(content string) ([]FinancialData, error) {
	var financialData []FinancialData

	// Извлекаем основную информацию о компании
	companyName := extractCompanyName(content)
	inn := extractINN(content)
	ogrn := extractOGRN(content)
	registrationDate := extractRegistrationDate(content)
	legalAddress := extractLegalAddress(content)
	director := extractDirector(content)
	authorizedCapital := extractAuthorizedFromSummaryCapital(content)

	fmt.Printf("Обработка данных для организации: %s (ИНН: %s, ОГРН: %s)\n", companyName, inn, ogrn)

	// Извлекаем финансовые данные по годам
	lines := strings.Split(content, "\n")
	
	// Собираем все года и соответствующие финансовые данные
	yearData := make(map[string]FinancialData)
	
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Ищем таблицу с финансовой отчетностью
		if strings.Contains(line, "Финансовая отчетность") || 
		   strings.Contains(line, "Выручка:") || 
		   strings.Contains(line, "Прибыль:") || 
		   strings.Contains(line, "Стоимость:") {
			
			// Просматриваем следующие строки для поиска данных по годам
			for j := i; j < len(lines) && j < i+10; j++ {
				currentLine := strings.TrimSpace(lines[j])
				if currentLine == "" {
					continue
				}

				// Ищем строки с годами и финансовыми показателями
				year, revenue, profit, cost := extractYearWithFinancialData(currentLine)
				if year != "" {
					yearData[year] = FinancialData{
						Year:              year,
						Revenue:           revenue,
						NetProfit:         profit,
						Cost:              getFloat64FromNull(cost),
						INN:               inn,
						OGRN:              ogrn,
						Name:              companyName,
						FullName:          companyName,
						RegistrationDate:  registrationDate,
						LegalAddress:      legalAddress,
						Director:          director,
						AuthorizedCapital: authorizedCapital,
						Date:              time.Now(),
					}
					fmt.Printf("Найдены данные за %s год: Выручка=%v, Прибыль=%v\n", 
						year, revenue, profit)
				}
			}
		}

		// Универсальный поиск: строки с финансовыми данными и годами
		if checkForFinancialData(line) {
			years := extractAllYears(line)
			
			for _, year := range years {
				revenue := extractFinancialValue(line, "Выручка:")
				profit := extractFinancialValue(line, "Прибыль:")
				cost := extractFinancialValue(line, "Стоимость:")

				if revenue.Valid || profit.Valid || cost.Valid {
					yearData[year] = FinancialData{
						Year:              year,
						Revenue:           revenue,
						NetProfit:         profit,
						Cost:              getFloat64FromNull(cost),
						INN:               inn,
						OGRN:              ogrn,
						Name:              companyName,
						FullName:          companyName,
						RegistrationDate:  registrationDate,
						LegalAddress:      legalAddress,
						Director:          director,
						AuthorizedCapital: authorizedCapital,
						Date:              time.Now(),
					}
				}
			}
		}
	}

	// Если не нашли данные через таблицу, используем общий поиск
	if len(yearData) == 0 {
		yearData = extractFinancialDataByPattern(lines, companyName, inn, ogrn, 
			registrationDate, legalAddress, director, authorizedCapital)
	}

	// Если вообще не найдено финансовых данных, создаем базовую запись с текущим годом
	if len(yearData) == 0 {
		currentYear := time.Now().Format("2006")
		yearData[currentYear] = FinancialData{
			Year:              currentYear,
			Revenue:           sql.NullFloat64{Valid: false},
			NetProfit:         sql.NullFloat64{Valid: false},
			Cost:              0,
			INN:               inn,
			OGRN:              ogrn,
			Name:              companyName,
			FullName:          companyName,
			RegistrationDate:  registrationDate,
			LegalAddress:      legalAddress,
			Director:          director,
			AuthorizedCapital: authorizedCapital,
			Date:              time.Now(),
		}
		fmt.Printf("Создана базовая запись ЕГРЮЛ за %s год (без финансовых показателей)\n", currentYear)
	}

	// Конвертируем map в slice
	for _, data := range yearData {
		financialData = append(financialData, data)
	}

	// Сортируем по году (от новых к старым)
	sort.Slice(financialData, func(i, j int) bool {
		return financialData[i].Year > financialData[j].Year
	})

	fmt.Printf("Всего извлечено финансовых записей: %d\n", len(financialData))
	return financialData, nil
}

// extractYearWithFinancialData извлекает год и финансовые данные из строки
func extractYearWithFinancialData(line string) (string, sql.NullFloat64, sql.NullFloat64, sql.NullFloat64) {
	var year string
	var revenue, profit, cost sql.NullFloat64

	// Ищем год в начале строки или в составе данных
	yearPattern := `\b(20\d{2})\b`
	re := regexp.MustCompile(yearPattern)
	matches := re.FindStringSubmatch(line)
	if len(matches) > 0 {
		year = matches[0]
	}

	if year == "" {
		return "", revenue, profit, cost
	}

	// Извлекаем финансовые показатели
	revenue = extractFinancialValue(line, "Выручка:")
	profit = extractFinancialValue(line, "Прибыль:")
	cost = extractFinancialValue(line, "Стоимость:")

	return year, revenue, profit, cost
}

// extractFinancialValue извлекает финансовое значение из строки
func extractFinancialValue(line, label string) sql.NullFloat64 {
	if !strings.Contains(line, label) {
		return sql.NullFloat64{Valid: false}
	}

	// Паттерн для поиска значений типа "59 млрд руб.", "8,6 млрд", "1.4 млрд"
	pattern := label + `\s*([\d,\.]+)\s*(млрд|млн|тыс)?`
	re := regexp.MustCompile(pattern)
	matches := re.FindStringSubmatch(line)
	
	if len(matches) >= 2 {
		valueStr := strings.ReplaceAll(matches[1], ",", ".")
		value, err := strconv.ParseFloat(valueStr, 64)
		if err != nil {
			return sql.NullFloat64{Valid: false}
		}

		// Конвертируем в рубли с учетом единиц измерения
		if len(matches) >= 3 {
			multiplier := getMultiplier(matches[2])
			value *= multiplier
		}

		return sql.NullFloat64{Float64: value, Valid: true}
	}

	return sql.NullFloat64{Valid: false}
}

// getMultiplier возвращает множитель для единиц измерения
func getMultiplier(unit string) float64 {
	switch unit {
	case "млрд":
		return 1000000000
	case "млн":
		return 1000000
	case "тыс":
		return 1000
	default:
		return 1
	}
}

// extractFinancialDataByPattern альтернативный метод извлечения данных по шаблонам
func extractFinancialDataByPattern(lines []string, companyName, inn, ogrn, 
	registrationDate, legalAddress, director string, authorizedCapital float64) map[string]FinancialData {
	
	yearData := make(map[string]FinancialData)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		
		// Универсальная проверка на наличие года в строке
		if strings.Contains(line, "Выручка:") && containsYear(line) {
			// Извлекаем все годы из строки
			years := extractAllYears(line)
			
			// Для каждого года в строке извлекаем финансовые данные
			for _, year := range years {
				revenue := extractFinancialValue(line, "Выручка:")
				profit := extractFinancialValue(line, "Прибыль:")
				cost := extractFinancialValue(line, "Стоимость:")

				if revenue.Valid || profit.Valid || cost.Valid {
					yearData[year] = FinancialData{
						Year:              year,
						Revenue:           revenue,
						NetProfit:         profit,
						Cost:              getFloat64FromNull(cost),
						INN:               inn,
						OGRN:              ogrn,
						Name:              companyName,
						FullName:          companyName,
						RegistrationDate:  registrationDate,
						LegalAddress:      legalAddress,
						Director:          director,
						AuthorizedCapital: authorizedCapital,
						Date:              time.Now(),
					}
					fmt.Printf("Найдены данные по шаблону за %s год: Выручка=%v, Прибыль=%v\n", 
						year, revenue, profit)
				}
			}
		}
	}

	return yearData
}

// extractAllYears извлекает все годы из строки
func extractAllYears(line string) []string {
	yearPattern := `\b(20\d{2})\b`
	re := regexp.MustCompile(yearPattern)
	matches := re.FindAllString(line, -1)
	
	// Убираем дубликаты
	uniqueYears := make(map[string]bool)
	for _, year := range matches {
		uniqueYears[year] = true
	}
	
	var result []string
	for year := range uniqueYears {
		result = append(result, year)
	}
	
	return result
}

// checkForFinancialData проверяет наличие финансовых данных с годами
func checkForFinancialData(line string) bool {
	return strings.Contains(line, "Выручка:") && containsYear(line)
}

// Вспомогательная функция для преобразования sql.NullFloat64 в float64
func getFloat64FromNull(nullFloat sql.NullFloat64) float64 {
	if nullFloat.Valid {
		return nullFloat.Float64
	}
	return 0.0
}

func extractCompanyName(content string) string {
	// Паттерны для поиска названия компании
	patterns := []string{
		`Полное наименование\s*\n-+\s*\n([^\n]+)`,
		`ООО "[^"]+"`,
		`ОБЩЕСТВО С ОГРАНИЧЕННОЙ ОТВЕТСТВЕННОСТЬЮ "[^"]+"`,
		`АО "[^"]+"`,
		`ПАО "[^"]+"`,
		`ЗАО "[^"]+"`,
		`НАО "[^"]+"`,
		`ИП [^\n]+`,
	}

	lines := strings.Split(content, "\n")
	
	// Поиск по паттернам
	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 && matches[1] != "" {
			return strings.TrimSpace(matches[1])
		} else if len(matches) > 0 {
			return strings.TrimSpace(matches[0])
		}
	}

	// Поиск по структуре отчета
	for i, line := range lines {
		// Ищем заголовок с названием компании
		if strings.Contains(line, "ООО") || strings.Contains(line, "АО") || 
		   strings.Contains(line, "ПАО") || strings.Contains(line, "ЗАО") ||
		   strings.Contains(line, "ИП") || strings.Contains(line, "Общество с ограниченной ответственностью") {
			
			// Проверяем, что это не просто упоминание в тексте
			if i > 0 && strings.Contains(strings.ToLower(lines[i-1]), "наименование") {
				return strings.TrimSpace(line)
			}
			
			// Если строка короткая и содержит кавычки - вероятно это название
			if len(line) < 100 && (strings.Contains(line, "\"") || strings.Contains(line, "«")) {
				return strings.TrimSpace(line)
			}
		}
	}

	return "Неизвестная компания"
}

func extractINN(content string) string {
	// Паттерны для поиска ИНН
	patterns := []string{
		`ИНН\s*(\d{10,12})`,
		`ИНН/КПП\s*(\d{10,12})`,
		`\b\d{10,12}\b`, // просто 10-12 цифр подряд
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			// Проверяем, что это действительно ИНН (10 или 12 цифр)
			inn := matches[1]
			if len(inn) == 10 || len(inn) == 12 {
				return inn
			}
		}
	}

	return ""
}

func extractOGRN(content string) string {
	// Паттерны для поиска ОГРН
	patterns := []string{
		`ОГРН\s*(\d{13,15})`,
		`ОГРНИП\s*(\d{15})`,
		`\b\d{13}\b`, // ОГРН юридического лица
		`\b\d{15}\b`, // ОГРНИП индивидуального предпринимателя
	}

	for _, pattern := range patterns {
		re := regexp.MustCompile(pattern)
		matches := re.FindStringSubmatch(content)
		if len(matches) > 1 {
			ogrn := matches[1]
			if len(ogrn) == 13 || len(ogrn) == 15 {
				return ogrn
			}
		}
	}

	return ""
}

// extractRegistrationDate извлекает дату регистрации
func extractRegistrationDate(content string) string {
	patterns := []string{
		`Дата регистрации\s*(\d{2}\.\d{2}\.\d{4})`,
		`Дата регистрации\s*(\d{1,2}\s+\w+\s+\d{4})`,
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

// extractLegalAddress извлекает юридический адрес
func extractLegalAddress(content string) string {
	patterns := []string{
		`Юридический адрес\s*([^\n]+)`,
		`Адрес[^\n]*:\s*([^\n]+)`,
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

// extractDirector извлекает информацию о директоре
func extractDirector(content string) string {
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

// extractAuthorizedFromSummaryCapital извлекает уставный капитал
func extractAuthorizedFromSummaryCapital(content string) float64 {
    patterns := []string{
		`уставный капитал\s*\d+\s+(\d[\d\s]*)`,
        `уставный капитал[^\d]*\d+[^\d]*(\d[\d\s]*)`,
        `размер.*рублях\)\s*\d+\s+(\d[\d\s]*)`,
        `размер.*рублях[^\d]*\d+[^\d]*(\d[\d\s]*)`,
        `Уставный капитал\s*\d+\s+(\d[\d\s]*)`,
        `УСТАВНЫЙ КАПИТАЛ\s*\d+\s+(\d[\d\s]*)`,
    }

    // Приводим всю строку к нижнему регистру для поиска
    contentLower := strings.ToLower(content)
    
    for _, pattern := range patterns {
        // Приводим паттерн к нижнему регистру
        patternLower := strings.ToLower(pattern)
        re := regexp.MustCompile(patternLower)
        matches := re.FindStringSubmatch(contentLower)

        if len(matches) > 1 {
            // Очищаем строку от пробелов и заменяем запятые на точки
            cleanStr := strings.ReplaceAll(strings.ReplaceAll(matches[1], " ", ""), ",", ".")

			
            
            // Удаляем возможные точки как разделители тысяч (если число формата 6.841.426.980)
            if strings.Count(cleanStr, ".") > 1 {
                cleanStr = strings.ReplaceAll(cleanStr, ".", "")
            }
			parts := strings.Fields(cleanStr)
            if len(parts) > 0 {
               cleanStr = parts[0]
            }
            
            if value, err := strconv.ParseFloat(cleanStr, 64); err == nil {
                return value
            }
            
            // Дополнительная попытка парсинга если есть проблема с форматом
            if strings.Contains(cleanStr, ".") {
                if value, err := strconv.ParseFloat(cleanStr, 64); err == nil {
                    return value
                }
            }
        }
    }
    
    // Альтернативный поиск по табличному формату из PDF
	altPatterns := []string{
        `размер.*рублях\)\s*([\d\s]+(?:[.,]\d+)?)`,
        `размер.*рублях[^\d]*([\d\s]+(?:[.,]\d+)?)`,
    }
    
    for _, pattern := range altPatterns {
        patternLower := strings.ToLower(pattern)
        re := regexp.MustCompile(patternLower)
        matches := re.FindStringSubmatch(contentLower)

        if len(matches) > 1 {
            cleanStr := strings.ReplaceAll(strings.ReplaceAll(matches[1], " ", ""), ",", ".")
            if strings.Count(cleanStr, ".") > 1 {
                cleanStr = strings.ReplaceAll(cleanStr, ".", "")
            }
			parts := strings.Fields(cleanStr)
            if len(parts) > 0 {
               cleanStr = parts[0]
            }
            
            if value, err := strconv.ParseFloat(cleanStr, 64); err != nil {
                return 0
            } else {
				return value
			}
        }
    }
    
    return 0
}

func extractYear(s string) string {
	for i := 0; i < len(s)-3; i++ {
		if s[i] >= '0' && s[i] <= '9' &&
			s[i+1] >= '0' && s[i+1] <= '9' &&
			s[i+2] >= '0' && s[i+2] <= '9' &&
			s[i+3] >= '0' && s[i+3] <= '9' {
			
			year, err := strconv.Atoi(s[i : i+4])
			if err == nil && year >= 2000 && year <= 2100 {
				return strconv.Itoa(year)
			}
		}
	}
	return ""
}

func findYearInContext(lines []string, currentIndex int) string {
	for i := currentIndex - 1; i >= 0 && i >= currentIndex-5; i-- {
		year := extractYear(lines[i])
		if year != "" {
			return year
		}
	}
	
	// Также проверяем следующие строки
	for i := currentIndex + 1; i < len(lines) && i <= currentIndex+5; i++ {
		year := extractYear(lines[i])
		if year != "" {
			return year
		}
	}
	
	return ""
}

func findFinancialLine(lines []string, yearIndex int) string {
	for i := yearIndex + 1; i < len(lines) && i <= yearIndex+3; i++ {
		if strings.Contains(lines[i], "Выручка:") ||
			strings.Contains(lines[i], "Прибыль:") ||
			strings.Contains(lines[i], "Стоимость:") {
			return lines[i]
		}
	}
	return ""
}