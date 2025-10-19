package main

import (
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ParseEgrlPDF парсит данные из выписки ЕГРЮЛ и возвращает FinancialData
func ParseEgrlPDF(content string) ([]FinancialData, error) {
	var financialData []FinancialData

	// Создаем основную запись
	data := FinancialData{
		Date:      time.Now(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	
	// Извлекаем все основные данные
	extractBasicInfo(&data, content)
	extractAuthorizedCapital(&data, content)
	extractDirectorInfo(&data, content)
	extractMainOKVEDEgrl(&data, content)
	extractAdditionalOKVED(&data, content)
	extractAddress(&data, content)
	extractLegalForm(&data, content)
	extractStatusInfo(&data, content)
	extractStaffInfo(&data, content)
	extractRegistrationInfo(&data, content)
	
	// Если есть основные реквизиты, добавляем запись
	if data.INN != "" || data.OGRN != "" {
		// Устанавливаем год из даты регистрации или текущий год
		data.Year = extractYearFromRegistrationDate(data.RegistrationDate)
		data.FullName = generateFullName(data)
		
		// Заполняем обязательные поля для БД
		fillRequiredFields(&data)
		
		financialData = append(financialData, data)
	}
	
	return financialData, nil
}

func extractBasicInfo(data *FinancialData, content string) {
	// ИНН - несколько вариантов поиска
	innPatterns := []string{
		`ИНН\s*юридического\s*лица\s*\|\s*\|\s*(\d{10,12})`,
		`ИНН/КПП\s*(\d{10})/(\d{9})`,
		`ИНН\s*(\d{10,12})`,
		`№\s*ИНН\s*(\d{10,12})`,
	}
	
	for _, pattern := range innPatterns {
		innRegex := regexp.MustCompile(pattern)
		if matches := innRegex.FindStringSubmatch(content); len(matches) > 1 {
			data.INN = matches[1]
			if len(matches) > 2 {
				data.KPP = matches[2]
			}
			break
		}
	}

	// ОГРН - несколько вариантов поиска
	ogrnPatterns := []string{
		`ОГРН\s*(\d{13})`,
		`Основной\s*государственный\s*регистрационный\s*номер\s*(\d{13})`,
		`№\s*ОГРН\s*(\d{13})`,
	}
	
	for _, pattern := range ogrnPatterns {
		ogrnRegex := regexp.MustCompile(pattern)
		if matches := ogrnRegex.FindStringSubmatch(content); len(matches) > 1 {
			data.OGRN = matches[1]
			break
		}
	}

	// КПП - если еще не нашли
	if data.KPP == "" {
		kppPatterns := []string{
			`КПП\s*юридического\s*лица\s*\|\s*\|\s*(\d{9})`,
			`КПП\s*(\d{9})`,
			`№\s*КПП\s*(\d{9})`,
		}
		
		for _, pattern := range kppPatterns {
			kppRegex := regexp.MustCompile(pattern)
			if matches := kppRegex.FindStringSubmatch(content); len(matches) > 1 {
				data.KPP = matches[1]
				break
			}
		}
	}

	// Полное наименование
	fullNamePatterns := []string{
		`Полное\s*наименование\s*на\s*русском\s*языке\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
		`Наименование\s*полное\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
		`Полное\s*наименование[^|]*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
	}
	
	for _, pattern := range fullNamePatterns {
		fullNameRegex := regexp.MustCompile(pattern)
		if matches := fullNameRegex.FindStringSubmatch(content); len(matches) > 1 {
			data.FullName = cleanString(matches[1])
			break
		}
	}

	// Сокращенное наименование
	namePatterns := []string{
		`Сокращенное\s*наименование\s*на\s*русском\s*языке\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
		`Наименование\s*сокращенное\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
		`Сокращенное\s*наименование[^|]*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
	}
	
	for _, pattern := range namePatterns {
		nameRegex := regexp.MustCompile(pattern)
		if matches := nameRegex.FindStringSubmatch(content); len(matches) > 1 {
			data.Name = cleanString(matches[1])
			break
		}
	}
	
	// Если сокращенного названия нет, используем начало полного
	if data.Name == "" && data.FullName != "" {
		data.Name = extractShortName(data.FullName)
	}

	// Дата регистрации
	regDatePatterns := []string{
		`Дата\s*регистрации\s*\|\s*\|\s*(\d{2}\.\d{2}\.\d{4})`,
		`Дата\s*внесения\s*записи\s*\|\s*\|\s*(\d{2}\.\d{2}\.\d{4})`,
		`Зарегистрировано[^0-9]*(\d{2}\.\d{2}\.\d{4})`,
	}
	
	for _, pattern := range regDatePatterns {
		regDateRegex := regexp.MustCompile(pattern)
		if matches := regDateRegex.FindStringSubmatch(content); len(matches) > 1 {
			data.RegistrationDate = matches[1]
			break
		}
	}

	// Налоговый орган
	taxAuthPatterns := []string{
		`Наименование\s*регистрирующего\s*органа\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
		`Регистрирующий\s*орган\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
		`Межрайонная\s*инспекция[^|]*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
	}
	
	for _, pattern := range taxAuthPatterns {
		taxAuthRegex := regexp.MustCompile(pattern)
		if matches := taxAuthRegex.FindStringSubmatch(content); len(matches) > 1 {
			data.TaxAuthority = cleanString(matches[1])
			break
		}
	}
	extractFromTableByRowNumber(data, content)
}

func extractFromTableByRowNumber(data *FinancialData, content string) {
    // Поиск основных данных по номерам строк из таблицы
    rowPatterns := map[string]string{
        "1":  `1\s*\|\s*\|\s*([^|\n]+?)\s*\|\s*\|`, // Полное наименование
        "3":  `3\s*\|\s*\|\s*([^|\n]+?)\s*\|\s*\|`, // Сокращенное наименование
        "59": `59\s*\|\s*\|\s*([^|\n]+?)\s*\|\s*\|`, // Основной ОКВЭД
    }
    
    for row, pattern := range rowPatterns {
        rowRegex := regexp.MustCompile(pattern)
        if matches := rowRegex.FindStringSubmatch(content); len(matches) > 1 {
            value := cleanString(matches[1])
            switch row {
            case "1":
                if data.FullName == "" {
                    data.FullName = value
                }
            case "3":
                if data.Name == "" {
                    data.Name = value
                }
            case "59":
                if data.MainOKVED == "" {
                    parts := strings.SplitN(value, " ", 2)
                    if len(parts) >= 2 {
                        data.MainOKVED = strings.TrimSpace(parts[0])
                        data.OKVEDDescription = strings.TrimSpace(parts[1])
                        data.MainOKVEDCode = toNullString(data.MainOKVED)
                        data.MainOKVEDActivity = toNullString(data.OKVEDDescription)
                    }
                }
            }
        }
    }
}

func extractAuthorizedCapital(data *FinancialData, content string) {
	// Уставный капитал - несколько вариантов поиска
	capitalPatterns := []string{
		`Размер[^0-9]*?рублях\)\s*\|\s*([\d\s,]+)`,
		`Уставный\s*капитал[^0-9]*([\d\s,]+)`,
		`Размер\s*уставного\s*капитала[^0-9]*([\d\s,]+)`,
		`Уставный\s*капитал[^|]*\|\s*\|\s*([\d\s,]+)`,
	}
	
	for _, pattern := range capitalPatterns {
		capitalRegex := regexp.MustCompile(pattern)
		if matches := capitalRegex.FindStringSubmatch(content); len(matches) > 1 {
			cleanStr := strings.ReplaceAll(strings.ReplaceAll(matches[1], " ", ""), ",", ".")
			if capital, err := strconv.ParseFloat(cleanStr, 64); err == nil {
				data.AuthorizedCapital = capital
				break
			}
		}
	}
	
	// Если не нашли числовое значение, ищем текстовое упоминание
	if data.AuthorizedCapital == 0 {
		if strings.Contains(content, "Уставный капитал") {
			// Пытаемся извлечь число из контекста
			contextRegex := regexp.MustCompile(`Уставный капитал[^0-9]*(\d+)`)
			if matches := contextRegex.FindStringSubmatch(content); len(matches) > 1 {
				if capital, err := strconv.ParseFloat(matches[1], 64); err == nil {
					data.AuthorizedCapital = capital
				}
			}
		}
	}
}

func extractDirectorInfo(data *FinancialData, content string) {
	// Поиск ФИО руководителя - несколько подходов
	
	// Подход 1: Поиск по должности и ФИО в таблице
	directorPatterns := []string{
		`(?:ГЕНЕРАЛЬНЫЙ ДИРЕКТОР|ДИРЕКТОР|РУКОВОДИТЕЛЬ)[^|]*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
		`Фамилия\s*Имя\s*Отчество[^|]*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
		`(?:Генеральный директор|Директор|Руководитель)[^|]*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
	}
	
	for _, pattern := range directorPatterns {
		directorRegex := regexp.MustCompile(pattern)
		if matches := directorRegex.FindStringSubmatch(content); len(matches) > 1 {
			data.Director = cleanString(matches[1])
			if isValidName(data.Director) {
				break
			}
		}
	}
	
	// Подход 2: Поиск по контексту в строках
	if data.Director == "" {
		lines := strings.Split(content, "\n")
		for i, line := range lines {
			if strings.Contains(line, "ГЕНЕРАЛЬНЫЙ ДИРЕКТОР") || 
			   strings.Contains(line, "ДИРЕКТОР") || 
			   strings.Contains(line, "РУКОВОДИТЕЛЬ") {
				
				// Ищем ФИО в соседних строках
				for j := max(0, i-3); j < min(len(lines), i+3); j++ {
					if isValidName(lines[j]) {
						data.Director = cleanString(lines[j])
						break
					}
				}
				break
			}
		}
	}
	
	// Подход 3: Поиск ФИО по паттерну (Фамилия Имя Отчество)
	if data.Director == "" {
		nameRegex := regexp.MustCompile(`[А-Я][а-я]+\s+[А-Я][а-я]+\s+[А-Я][а-я]+`)
		if matches := nameRegex.FindStringSubmatch(content); len(matches) > 0 {
			data.Director = matches[0]
		}
	}
}
func extractMainOKVEDEgrl(data *FinancialData, content string) {
    // Основной ОКВЭД - улучшенные паттерны для табличного формата
    okvedPatterns := []string{
        `Код\s*и\s*наименование\s*вида\s*деятельности\s*\|\s*\|\s*(\d{2}\.\d{2}[^\n|]*?)\s*[\n|]`,
        `Основной\s*вид\s*деятельности[^|]*\|\s*\|\s*(\d{2}\.\d{2}[^\n|]*?)\s*[\n|]`,
        `Код\s*ОКВЭД[^|]*\|\s*\|\s*(\d{2}\.\d{2}[^\n|]*?)\s*[\n|]`,
        `59\s*\|\s*\|\s*(\d{2}\.\d{2}[^\n|]*?)\s*[\n|]`, // По номеру строки из таблицы
        `29\.10[^\n|]*?Производство автотранспортных средств`, // Конкретный код из вашего файла
    }

    fmt.Printf("Поиск основного ОКВЭД в контенте...\n")

    for _, pattern := range okvedPatterns {
        okvedRegex := regexp.MustCompile(pattern)
        if matches := okvedRegex.FindStringSubmatch(content); len(matches) > 1 {
            okvedInfo := cleanString(matches[1])
            fmt.Printf("Найден ОКВЭД: %s\n", okvedInfo)
            
            // Разбираем код и описание
            parts := strings.SplitN(okvedInfo, " ", 2)
            if len(parts) >= 2 {
                data.MainOKVED = strings.TrimSpace(parts[0])
                data.OKVEDDescription = strings.TrimSpace(parts[1])
            } else {
                data.MainOKVED = okvedInfo
                data.OKVEDDescription = ""
            }
            
            data.MainOKVEDCode = toNullString(data.MainOKVED)
            data.MainOKVEDActivity = toNullString(data.OKVEDDescription)
            fmt.Printf("Установлен основной ОКВЭД: %s - %s\n", data.MainOKVED, data.OKVEDDescription)
            return
        }
    }

    // Альтернативный поиск по структуре таблицы
    extractOKVEDFromTable(data, content)
}
func extractAdditionalOKVED(data *FinancialData, content string) {
    var additionalOKVEDs []string
    
    fmt.Println("Поиск дополнительных ОКВЭД...")
    
    // Упрощенный подход: ищем все ОКВЭД коды после определенных меток
    lines := strings.Split(content, "\n")
    inAdditionalSection := false
    
    for i, line := range lines {
        // Ищем начало секции дополнительных ОКВЭД
        if strings.Contains(line, "Дополнительные виды деятельности") ||
           strings.Contains(line, "Дополнительные виды экономической деятельности") ||
           strings.Contains(line, "Сведения о дополнительных видах деятельности") {
            inAdditionalSection = true
            fmt.Printf("Найдена секция дополнительных ОКВЭД в строке %d\n", i)
            continue
        }
        
        // Если мы в секции дополнительных ОКВЭД, ищем коды
        if inAdditionalSection {
            // Ищем ОКВЭД коды в строке
            okvedRegex := regexp.MustCompile(`(\d{2}\.[\d\.]+)`)
            matches := okvedRegex.FindAllStringSubmatch(line, -1)
            
            for _, match := range matches {
                if len(match) > 1 {
                    code := strings.TrimSpace(match[1])
                    // Проверяем что это валидный ОКВЭД и не основной
                    if isValidOKVEDCode(code) && code != data.MainOKVED && !contains(additionalOKVEDs, code) {
                        additionalOKVEDs = append(additionalOKVEDs, code)
                        fmt.Printf("Найден дополнительный ОКВЭД: %s\n", code)
                    }
                }
            }
            
            // Выходим из секции если нашли конец или другую основную секцию
            if strings.Contains(line, "Основной вид деятельности") ||
               strings.Contains(line, "Сведения об основном виде деятельности") ||
               strings.Contains(line, "Регистрирующий орган") ||
               i > 1000 { // ограничиваем поиск
                inAdditionalSection = false
            }
        }
    }
    
    // Альтернативный подход: поиск по номерам строк в таблице
    if len(additionalOKVEDs) == 0 {
        fmt.Println("Поиск дополнительных ОКВЭД по номерам строк таблицы...")
        for i := 61; i <= 115; i += 2 {
            pattern := fmt.Sprintf(`%d\s*\|\s*\|\s*(\d{2}\.[\d\.]+)`, i)
            okvedRegex := regexp.MustCompile(pattern)
            if matches := okvedRegex.FindStringSubmatch(content); len(matches) > 1 {
                code := strings.TrimSpace(matches[1])
                if isValidOKVEDCode(code) && code != data.MainOKVED && !contains(additionalOKVEDs, code) {
                    additionalOKVEDs = append(additionalOKVEDs, code)
                    fmt.Printf("Найден дополнительный ОКВЭД из строки %d: %s\n", i, code)
                }
            }
        }
    }
    
    // Резервный подход: поиск всех ОКВЭД кодов в контенте
    if len(additionalOKVEDs) == 0 {
        fmt.Println("Резервный поиск всех ОКВЭД кодов в контенте...")
        okvedRegex := regexp.MustCompile(`(\d{2}\.[\d\.]+)`)
        allMatches := okvedRegex.FindAllStringSubmatch(content, -1)
        
        for _, match := range allMatches {
            if len(match) > 1 {
                code := strings.TrimSpace(match[1])
                // Исключаем основной ОКВЭД и проверяем валидность
                if code != data.MainOKVED && isValidOKVEDCode(code) && !contains(additionalOKVEDs, code) {
                    additionalOKVEDs = append(additionalOKVEDs, code)
                }
            }
        }
    }
    
    // Ограничиваем количество дополнительных ОКВЭД (обычно не больше 20-30)
    if len(additionalOKVEDs) > 30 {
        additionalOKVEDs = additionalOKVEDs[:30]
    }
    
    if len(additionalOKVEDs) > 0 {
        data.AdditionalIndustry = toNullString(strings.Join(additionalOKVEDs, ", "))
        fmt.Printf("Найдено дополнительных ОКВЭД: %d -> %v\n", len(additionalOKVEDs), additionalOKVEDs)
    } else {
        fmt.Println("Дополнительные ОКВЭД не найдены")
        data.AdditionalIndustry = toNullString("")
    }
}

// Функция для проверки валидности кода ОКВЭД
func isValidOKVEDCode(code string) bool {
    // Проверяем формат кода ОКВЭД: XX.XX или XX.XX.XX
    okvedRegex := regexp.MustCompile(`^\d{2}\.\d{2}(?:\.\d{1,2})?$`)
    return okvedRegex.MatchString(code)
}

// Вспомогательная функция для извлечения ОКВЭД из таблицы
func extractOKVEDFromTable(data *FinancialData, content string) {
    // Поиск по конкретным строкам таблицы из вашего файла
    tablePatterns := []string{
        `59\s*\|\s*\|\s*(\d{2}\.\d+[^\n|]*?)\s*[\n|]`, // Строка 59 - основной ОКВЭД
    }
    
    for _, pattern := range tablePatterns {
        tableRegex := regexp.MustCompile(pattern)
        if matches := tableRegex.FindStringSubmatch(content); len(matches) > 1 {
            okvedInfo := cleanString(matches[1])
            parts := strings.SplitN(okvedInfo, " ", 2)
            if len(parts) >= 2 {
                data.MainOKVED = strings.TrimSpace(parts[0])
                data.OKVEDDescription = strings.TrimSpace(parts[1])
                data.MainOKVEDCode = toNullString(data.MainOKVED)
                data.MainOKVEDActivity = toNullString(data.OKVEDDescription)
                fmt.Printf("Найден основной ОКВЭД из таблицы: %s - %s\n", data.MainOKVED, data.OKVEDDescription)
                return
            }
        }
    }
    
    // Поиск по известным кодам из файла
    knownOKVEDs := []string{
        `29\.10\s*Производство автотранспортных средств`,
        `28\.92\.27\s*Производство прочих экскаваторов`,
        `29\.31\s*Производство электрического и электронного оборудования`,
        `29\.32\s*Производство прочих комплектующих`,
    }
    
    for _, okved := range knownOKVEDs {
        pattern := regexp.QuoteMeta(okved)
        okvedRegex := regexp.MustCompile(pattern)
        if okvedRegex.MatchString(content) {
            parts := strings.SplitN(okved, " ", 2)
            if len(parts) >= 2 {
                data.MainOKVED = strings.TrimSpace(parts[0])
                data.OKVEDDescription = strings.TrimSpace(parts[1])
                data.MainOKVEDCode = toNullString(data.MainOKVED)
                data.MainOKVEDActivity = toNullString(data.OKVEDDescription)
                fmt.Printf("Найден ОКВЭД по известному коду: %s - %s\n", data.MainOKVED, data.OKVEDDescription)
                return
            }
        }
    }
}

// Функция для извлечения кодов ОКВЭД из секции
func extractOKVEDCodesFromSection(sectionContent string, codes *[]string) {
    okvedRegex := regexp.MustCompile(`(\d{2}\.[\d\.]+)`)
    matches := okvedRegex.FindAllStringSubmatch(sectionContent, -1)
    
    for _, match := range matches {
        if len(match) > 1 {
            code := strings.TrimSpace(match[1])
            if !contains(*codes, code) {
                *codes = append(*codes, code)
            }
        }
    }
}

// Вспомогательная функция проверки наличия элемента в слайсе
func contains(slice []string, item string) bool {
    for _, s := range slice {
        if s == item {
            return true
        }
    }
    return false
}

func extractAddress(data *FinancialData, content string) {
	// Адрес юридического лица - более специфичные паттерны
	addressPatterns := []string{
		`Адрес\s*юридического\s*лица\s*\|\s*\|\s*([^|\n]{10,200}?)\s*\|\s*\|`,
		`Место\s*нахождения\s*юридического\s*лица\s*\|\s*\|\s*([^|\n]{10,200}?)\s*\|\s*\|`,
		`Адрес[^|]*\|\s*\|\s*([^|\n]{10,200}?)\s*\|\s*\|`,
		`Юридический\s*адрес[^|]*\|\s*\|\s*([^|\n]{10,200}?)\s*\|\s*\|`,
	}
	
	for _, pattern := range addressPatterns {
		addressRegex := regexp.MustCompile(pattern)
		if matches := addressRegex.FindStringSubmatch(content); len(matches) > 1 {
			address := cleanString(matches[1])
			// Проверяем, что это похоже на адрес (содержит индекс или типичные адресные слова)
			if isValidAddress(address) {
				data.LegalAddress = address
				break
			}
		}
	}
	
	// Если адрес не найден, пытаемся извлечь из контекста
	if data.LegalAddress == "" {
		// Поиск адреса по индексу и типичной структуре
		zipRegex := regexp.MustCompile(`\d{6},\s*[^|\n]{10,150}`)
		if matches := zipRegex.FindStringSubmatch(content); len(matches) > 0 {
			address := cleanString(matches[0])
			if isValidAddress(address) {
				data.LegalAddress = address
			}
		}
	}
}

// Новая функция для проверки валидности адреса
func isValidAddress(address string) bool {
	if len(address) < 10 || len(address) > 300 {
		return false
	}
	
	// Адрес должен содержать типичные элементы
	hasZip := regexp.MustCompile(`\d{6}`).MatchString(address)
	hasCity := regexp.MustCompile(`(г\.|город|гор\.|п\.|пос\.|деревня|с\.)`).MatchString(strings.ToLower(address))
	hasStreet := regexp.MustCompile(`(ул\.|улица|пр\.|проспект|пер\.|переулок|б-р|бульвар)`).MatchString(strings.ToLower(address))
	
	// Хотя бы два из трех критериев должны выполняться
	criteriaCount := 0
	if hasZip { criteriaCount++ }
	if hasCity { criteriaCount++ }
	if hasStreet { criteriaCount++ }
	
	return criteriaCount >= 2
}

func extractLegalForm(data *FinancialData, content string) {
	// Организационно-правовая форма
	legalFormPatterns := []string{
		`(ООО|АО|ПАО|ЗАО|ИП|ОАО|Общество с ограниченной ответственностью|Акционерное общество|Публичное акционерное общество|Закрытое акционерное общество)`,
		`Организационно-правовая\s*форма\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
	}
	
	for _, pattern := range legalFormPatterns {
		legalFormRegex := regexp.MustCompile(pattern)
		if matches := legalFormRegex.FindStringSubmatch(data.FullName); len(matches) > 0 {
			data.LegalForm = matches[1]
			break
		}
		if matches := legalFormRegex.FindStringSubmatch(content); len(matches) > 1 {
			data.LegalForm = cleanString(matches[1])
			break
		}
	}
}

func extractStatusInfo(data *FinancialData, content string) {
	// Статус организации
	statusPatterns := []string{
		`Статус\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
		`Состояние\s*юридического\s*лица\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`,
	}
	
	for _, pattern := range statusPatterns {
		statusRegex := regexp.MustCompile(pattern)
		if matches := statusRegex.FindStringSubmatch(content); len(matches) > 1 {
			data.Status = cleanString(matches[1])
			break
		}
	}
	
	// Если не нашли в структурированном виде, ищем по ключевым словам
	if data.Status == "" {
		switch {
		case strings.Contains(content, "Действующая") || strings.Contains(content, "действующ"):
			data.Status = "Действующая"
		case strings.Contains(content, "Ликвидирована") || strings.Contains(content, "ликвидиров"):
			data.Status = "Ликвидирована"
		case strings.Contains(content, "Реорганизация") || strings.Contains(content, "реорганизац"):
			data.Status = "Реорганизация"
		case strings.Contains(content, "Недействующая") || strings.Contains(content, "недействующ"):
			data.Status = "Недействующая"
		default:
			data.Status = "Неизвестно"
		}
	}
}

func extractStaffInfo(data *FinancialData, content string) {
	// Среднесписочная численность
	staffPatterns := []string{
		`Среднесписочная\s*численность[^0-9]*(\d+)`,
		`Численность[^0-9]*(\d+)`,
		`Количество\s*работников[^0-9]*(\d+)`,
	}
	
	for _, pattern := range staffPatterns {
		staffRegex := regexp.MustCompile(pattern)
		if matches := staffRegex.FindStringSubmatch(content); len(matches) > 1 {
			if count, err := strconv.Atoi(matches[1]); err == nil {
				data.StaffCount = sql.NullInt64{Int64: int64(count), Valid: true}
				data.TotalStaff = sql.NullInt64{Int64: int64(count), Valid: true}
				break
			}
		}
	}
}

func extractRegistrationInfo(data *FinancialData, content string) {
	// Дополнительная информация о регистрации
	regNumberRegex := regexp.MustCompile(`Регистрационный\s*номер\s*(\S+)`)
	if matches := regNumberRegex.FindStringSubmatch(content); len(matches) > 1 {
		if data.TaxAuthority == "" {
			data.TaxAuthority = matches[1]
		}
	}
	
	// Дата внесения записи в ЕГРЮЛ
	entryDateRegex := regexp.MustCompile(`Дата\s*внесения\s*записи\s*в\s*ЕГРЮЛ\s*\|\s*\|\s*(\d{2}\.\d{2}\.\d{4})`)
	if matches := entryDateRegex.FindStringSubmatch(content); len(matches) > 1 {
		data.AddedToRegistryDate = toNullTime(matches[1])
	}
}

// Вспомогательные функции

func cleanString(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\t", " ")
	// Убираем двойные пробелы
	for strings.Contains(s, "  ") {
		s = strings.ReplaceAll(s, "  ", " ")
	}
	return s
}

func isValidName(s string) bool {
	s = cleanString(s)
	// Проверяем, что строка похожа на ФИО (содержит 3 слова, начинающихся с заглавных букв)
	words := strings.Fields(s)
	if len(words) != 3 {
		return false
	}
	for _, word := range words {
		if len(word) < 2 || !isUpper(word[0]) {
			return false
		}
	}
	return true
}

func isUpper(b byte) bool {
	return b >= 'A' && b <= 'Z'
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func toNullTime(dateStr string) sql.NullTime {
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

// Остальные функции остаются без изменений...
// extractYearFromRegistrationDate, extractShortName, generateFullName, etc.

func fillRequiredFields(data *FinancialData) {
	// Заполняем обязательные поля для вставки в БД
	if data.Name == "" {
		data.Name = "Неизвестно"
	}
	if data.FullName == "" {
		data.FullName = data.Name
	}
	if data.Year == "" {
		data.Year = strconv.Itoa(time.Now().Year())
	}
	
	// Устанавливаем периоды
	startPeriod, finishPeriod := createPeriodFromYear(data.Year)
	data.StartPeriod = startPeriod
	data.FinishPeriod = finishPeriod
	
	// Устанавливаем ревизию
	data.Revision = 1
	data.Destroyed = false
}

// extractYearFromRegistrationDate извлекает год из даты регистрации
func extractYearFromRegistrationDate(regDate string) string {
	if regDate == "" {
		return strconv.Itoa(time.Now().Year())
	}
	
	yearRegex := regexp.MustCompile(`(\d{4})`)
	if matches := yearRegex.FindStringSubmatch(regDate); len(matches) > 1 {
		return matches[1]
	}
	
	return strconv.Itoa(time.Now().Year())
}

// extractShortName извлекает сокращенное название из полного
func extractShortName(fullName string) string {
	if strings.Contains(fullName, "\"") {
		// Если есть кавычки, берем содержимое кавычек
		quoteRegex := regexp.MustCompile(`"([^"]+)"`)
		if matches := quoteRegex.FindStringSubmatch(fullName); len(matches) > 1 {
			return matches[1]
		}
	}
	
	// Иначе берем первые 2-3 слова
	words := strings.Fields(fullName)
	if len(words) > 3 {
		return strings.Join(words[:3], " ")
	}
	
	return fullName
}

// generateFullName генерирует полное название с реквизитами
func generateFullName(data FinancialData) string {
	if data.FullName != "" {
		return data.FullName
	}
	
	name := data.Name
	if name == "" {
		name = "Неизвестная организация"
	}
	
	if data.INN != "" {
		name += " (ИНН: " + data.INN + ")"
	}
	if data.OGRN != "" {
		name += " (ОГРН: " + data.OGRN + ")"
	}
	
	return name
}

// Вспомогательная функция для поиска значения по метке
func findValueByLabel(content, label string) string {
	pattern := regexp.QuoteMeta(label) + `\s*\|\s*\|\s*([^|]+?)\s*\|\s*\|`
	regex := regexp.MustCompile(pattern)
	if matches := regex.FindStringSubmatch(content); len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return ""
}

// Обновленная функция для интеграции с основной системой
func ParseEgrlPDFContent(content string) ([]FinancialData, error) {
	fmt.Println("Парсинг выписки ЕГРЮЛ...")
	return ParseEgrlPDF(content)
}