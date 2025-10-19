package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

// Структура для JSON ответа
type UploadResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Data    struct {
		FileName     string `json:"file_name,omitempty"`
		OriginalName string `json:"original_name,omitempty"`
		DocID        int64  `json:"doc_id,omitempty"`
		InsertedCount int   `json:"inserted_count,omitempty"`
	} `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
}

func uploadFileHandler(w http.ResponseWriter, r *http.Request) {
	// Устанавливаем заголовки для JSON ответа
	w.Header().Set("Content-Type", "application/json; charset=utf-8")

	response := UploadResponse{}

	if r.Method != "POST" {
		response.Success = false
		response.Error = "Метод не поддерживается"
		w.WriteHeader(http.StatusMethodNotAllowed)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Парсим multipart форму (максимальный размер 10MB)
	if err := r.ParseMultipartForm(10 << 20); err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("Ошибка чтения формы: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Получаем файл из формы
	file, header, err := r.FormFile("file")
	if err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("Ошибка получения файла: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}
	defer file.Close()

	// Проверяем расширение файла (только PDF и Excel)
	ext := filepath.Ext(header.Filename)
	if !isAllowedExtension(ext) {
		response.Success = false
		response.Error = "Недопустимый тип файла. Разрешены только: PDF (.pdf), Excel (.xlsx, .xls)"
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Генерируем уникальное имя файла
	newFileName := generateFileName(header.Filename)
	filePath := filepath.Join(uploadDir, newFileName)

	// Создаем файл на сервере
	dst, err := os.Create(filePath)
	if err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("Ошибка создания файла: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	defer dst.Close()

	// Копируем содержимое загруженного файла
	if _, err := io.Copy(dst, file); err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("Ошибка сохранения файла: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Получаем информацию о файле и определяем тип
	doc, err := getFileInfo(filePath, header.Filename, newFileName)
	if err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("Ошибка анализа файла: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Проверяем подключение к БД
	if err := db.Ping(); err != nil {
		// Пытаемся переподключиться
		newDB, err := connectToDB()
		if err != nil {
			response.Success = false
			response.Error = "Ошибка подключения к БД. Попробуйте позже."
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(response)
			return
		}
		db = newDB
	}

	// Сохраняем документ в БД
	docID, err := SaveDocument(db, doc)
	if err != nil {
		response.Success = false
		response.Error = fmt.Sprintf("Ошибка сохранения в БД: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}

	// Определяем тип файла
	fileType, err := detectFileType(filePath, header.Filename)
	if err != nil {
		log.Printf("Ошибка определения типа файла: %v", err)
		// Не прерываем выполнение, продолжаем без финансовых данных
	} else {
		fmt.Printf("Тип файла: %v\n", fileType)

		// Чтение данных из файла (только для финансовых файлов)
		financialData, err := readFinancialFile(filePath, header.Filename, fileType)
		if err != nil {
			log.Printf("Ошибка чтения файла: %v", err)
		} else {
			// Устанавливаем DocID и DocType для финансовых данных
			for i := range financialData {
				financialData[i].DocID = docID
				financialData[i].DocType = doc.DocumentTypeID
			}

			// Вставка данных в таблицу
			insertedCount, err := InsertFinancialData(db, financialData)
			if err != nil {
				log.Printf("Ошибка вставки данных: %v", err)
			} else {
				fmt.Printf("Успешно добавлено %d записей в базу данных\n", insertedCount)
				response.Data.InsertedCount = insertedCount
			}
		}
	}

	// Формируем успешный ответ
	response.Success = true
	response.Message = "Файл успешно загружен и обработан"
	response.Data.FileName = newFileName
	response.Data.OriginalName = header.Filename
	response.Data.DocID = docID

	// Отправляем успешный ответ
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(response)
}

// Вспомогательная функция для отправки ошибок в JSON формате
func sendJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	response := UploadResponse{
		Success: false,
		Error:   message,
	}
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}