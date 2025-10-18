package main

import (
	"context"
	"io"
	"log"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"industrialregistrysystem/base/api"
)

func main() {
	// Data Service - активный клиент, готовый обрабатывать запросы
	dataService := NewDataService()
	
	// Бесконечный цикл для переподключения
	for {
		err := connectAndServe(dataService)
		if err != nil {
			log.Printf("Connection failed: %v. Reconnecting in 5 seconds...", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func connectAndServe(dataService *DataService) error {
	// Подключаемся к gRPC серверу на localhost:5051 с TLS
	tlsCredentials, err := loadTLSCredentialsClient()
	if err != nil {
		log.Printf("❌ Failed to load TLS credentials, using insecure: %v", err)
		// Fallback to insecure connection
		connection, err := grpc.Dial("localhost:5051", grpc.WithTransportCredentials(insecure.NewCredentials()))
		if err != nil {
			return err
		}
		defer connection.Close()
		
		return serveWithConnection(dataService, connection)
	}

	// Используем TLS соединение
	connection, err := grpc.Dial("localhost:5051", grpc.WithTransportCredentials(tlsCredentials))
	if err != nil {
		return err
	}
	defer connection.Close()
	
	return serveWithConnection(dataService, connection)
}

func serveWithConnection(dataService *DataService, connection *grpc.ClientConn) error {
	// Создаем gRPC клиент
	client := api.NewDatabaseServiceClient(connection)
	
	log.Println("📊 Data Service (Active gRPC Client) started - ready to handle DB requests")
	log.Println("   Connected to PostgreSQL database")
	log.Println("   Connected to gRPC server on localhost:5051")
	log.Println("   Establishing command channel...")
	
	// Устанавливаем streaming соединение
	ctx := context.Background()
	stream, err := client.CommandStream(ctx)
	if err != nil {
		return err
	}
	
	// Отправляем initial ready message
	err = stream.Send(&api.CommandResponse{
		RequestId: "ready",
		Response: &api.CommandResponse_Ready{
			Ready: &api.ReadyMessage{
				ServiceName: "database-service",
			},
		},
	})
	if err != nil {
		return err
	}
	
	log.Println("✅ Command channel established - waiting for server commands...")
	
	// Обрабатываем входящие команды от сервера
	for {
		command, err := stream.Recv()
		if err == io.EOF {
			log.Println("Server closed connection")
			return nil
		}
		if err != nil {
			return err
		}
		
		// Обрабатываем команду в горутине чтобы не блокировать получение новых команд
		go handleServerCommand(dataService, stream, command)
	}
}