package main

import (
	"os"
	"fmt"
	"path/filepath"
	"crypto/rand"
	"time"
)

// generateFileName создает уникальное имя файла
func generateFileName(originalName string) string {
    ext := filepath.Ext(originalName)
    
    // Генерируем UUID v4
    uuid := make([]byte, 16)
    _, err := rand.Read(uuid)
    if err != nil {
        // Fallback: используем timestamp если crypto/rand не работает
        timestamp := time.Now().UnixNano()
        return fmt.Sprintf("%d%s", timestamp, ext)
    }
    
    // UUID v4 спецификация
    uuid[6] = (uuid[6] & 0x0f) | 0x40 // Version 4
    uuid[8] = (uuid[8] & 0x3f) | 0x80 // Variant is 10
    
    // Преобразуем в строку без дефисов
    guid := fmt.Sprintf("%x%x%x%x%x", 
        uuid[0:4], uuid[4:6], uuid[6:8], uuid[8:10], uuid[10:16])
    
    return guid + ext
}

// isAllowedExtension проверяет только те расширения, которые может обработать detectFileType
func isAllowedExtension(ext string) bool {
	allowed := map[string]bool{
		".pdf":  true,  // PDF файлы (Summary, Due Diligence, Financial Report)
		".xlsx": true,  // Excel файлы (MultiYear, SingleYear)
		".xls":  true,  // Excel файлы (старый формат)
	}
	return allowed[ext]
}

// getFileInfo возвращает информацию о файле для сохранения в БД
func getFileInfo(filePath string, originalName string, newFileName string) (*Document, error) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return nil, err
	}

	fileType, err := detectFileType(filePath, originalName)
	if err != nil {
		return nil, err
	}

	doc := &Document{
		FilePath:         filePath,
		FileName:         newFileName,
		FileExtension:    filepath.Ext(originalName),
		OriginalFileName: originalName,
		FileSize:         fileInfo.Size(),
		DocumentTypeID:   int64(fileType),
		CreatedAt:        time.Now(),
	}

	return doc, nil
}