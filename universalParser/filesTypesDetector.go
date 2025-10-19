package main

import (
    "fmt"
    "path/filepath"
    "strings"

    "github.com/xuri/excelize/v2"
    _ "github.com/lib/pq"
)

// detectFileType определяет тип файла по расширению и содержанию
func detectFileType(filePath string, originalName string) (FileType, error) {
    if !fileExists(filePath) {
        return FileTypeUnknown, fmt.Errorf("файл не существует: %s", filePath)
    }

    ext := strings.ToLower(filepath.Ext(filePath))
    
    switch ext {
    case ".xlsx", ".xls":
        return detectExcelFileType(filePath, originalName)
    case ".pdf":
        return detectPDFFileType(filePath, originalName)
    default:
        return FileTypeUnknown, fmt.Errorf("неподдерживаемый формат файла: %s", ext)
    }
}

// detectExcelFileType определяет тип Excel файла
func detectExcelFileType(filePath string, originalName string) (FileType, error) {
    if !fileExists(filePath) {
        return FileTypeUnknown, fmt.Errorf("файл не существует: %s", filePath)
    }

    f, err := excelize.OpenFile(filePath)
    if err != nil {
        return FileTypeUnknown, err
    }
    defer f.Close()

    // Пробуем прочитать баланс
    rows, err := f.GetRows("Бухгалтерский баланс")
    if err != nil {
        return FileTypeUnknown, err
    }

    if len(rows) == 0 {
        return FileTypeUnknown, fmt.Errorf("файл не содержит данных")
    }

    // Анализируем заголовки
    headerRow := rows[0]
    yearCount := 0
    
    for _, cell := range headerRow {
        if isYear(cell) {
            yearCount++
        }
    }

    if yearCount > 1 {
        return FileTypeExcelMultiYear, nil
    } else if yearCount == 1 {
        return FileTypeExcelSingleYear, nil
    }

    // Если годы не найдены в заголовках, проверяем наличие данных
    if len(rows) > 1 && len(rows[1]) > 2 {
        return FileTypeExcelSingleYear, nil
    }

    return FileTypeUnknown, fmt.Errorf("не удалось определить формат Excel файла")
}

// detectPDFFileType определяет тип PDF файла
func detectPDFFileType(filePath string, originalName string) (FileType, error) {
    if !fileExists(filePath) {
        return FileTypeUnknown, fmt.Errorf("файл не существует: %s\n", filePath)
    }

    filename := strings.ToLower(originalName)
    
    if strings.Contains(filename, "сводный") || strings.Contains(filename, "сводный отчет") {
        return FileTypePDFSummary, nil
    } else if strings.Contains(filename, "должной") || strings.Contains(filename, "осмотрительности") {
        return FileTypePDFDueDiligence, nil
    } else if strings.Contains(filename, "егрюл") {
        return FinancialReport, nil
    }
    return FileTypePDFSummary, nil
}
