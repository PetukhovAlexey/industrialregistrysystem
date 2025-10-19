
package main

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
)

const (
	uploadDir = "./uploads"
	staticDir = "./static"
	port      = ":8080"
)

func runServer() {
	// Создаем необходимые директории
	if err := os.MkdirAll(uploadDir, 0755); err != nil {
		log.Fatal("Ошибка создания директории uploads:", err)
	}
	if err := os.MkdirAll(staticDir, 0755); err != nil {
		log.Fatal("Ошибка создания директории static:", err)
	}

	// Настраиваем маршруты
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/upload", uploadFileHandler)
	http.Handle("/download/", http.StripPrefix("/download/", http.FileServer(http.Dir(uploadDir))))

	log.Printf("Сервер запущен на http://localhost%s", port)
	log.Printf("Статические файлы обслуживаются из: %s", staticDir)
	log.Fatal(http.ListenAndServe(port, nil))
}

// Обработчик главной страницы
func indexHandler(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	// Отдаем index.html из статической папки
	http.ServeFile(w, r, filepath.Join(staticDir, "index.html"))
}

// Показывает страницу успеха
func showSuccessPage(w http.ResponseWriter, fileName, originalName string) {
	tmplPath := filepath.Join(staticDir, "success.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		http.Error(w, "Ошибка загрузки шаблона", http.StatusInternalServerError)
		return
	}

	data := struct {
		FileName     string
		OriginalName string
	}{
		FileName:     fileName,
		OriginalName: originalName,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}

// Показывает страницу ошибки
func showErrorPage(w http.ResponseWriter, errorMessage string) {
	tmplPath := filepath.Join(staticDir, "error.html")
	tmpl, err := template.ParseFiles(tmplPath)
	if err != nil {
		// Если шаблон ошибки не найден, показываем простую ошибку
		http.Error(w, errorMessage, http.StatusInternalServerError)
		return
	}

	data := struct {
		Error string
	}{
		Error: errorMessage,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	tmpl.Execute(w, data)
}