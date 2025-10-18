package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log"
	"os"
	"time"

	"google.golang.org/grpc/credentials"
	"industrialregistrysystem/base/api"
)

// loadTLSCredentialsClient загружает TLS сертификаты для клиента
func loadTLSCredentialsClient() (credentials.TransportCredentials, error) {
	// Загружаем клиентский сертификат
	clientCertificate, err := tls.LoadX509KeyPair("certs/database/database-fullchain.crt", "certs/database/database.key")
	if err != nil {
		return nil, fmt.Errorf("failed to load client certificates: %v", err)
	}

	// Загружаем CA сертификат
	caCertificate, err := os.ReadFile("certs/ca/ca.crt")
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate: %v", err)
	}

	certificatePool := x509.NewCertPool()
	if !certificatePool.AppendCertsFromPEM(caCertificate) {
		return nil, fmt.Errorf("failed to add CA certificate to pool")
	}

	// Настраиваем TLS конфигурацию
	configuration := &tls.Config{
		Certificates: []tls.Certificate{clientCertificate},
		RootCAs:      certificatePool,
		ServerName:   "mainservice", // Должен совпадать с CN сертификата сервера
		MinVersion:   tls.VersionTLS12,
	}

	return credentials.NewTLS(configuration), nil
}

// handleServerCommand обрабатывает команды от сервера
func handleServerCommand(dataService *DataService, stream api.DatabaseService_CommandStreamClient, command *api.CommandRequest) {
	contextWithTimeout, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	
	var response *api.CommandResponse
	
	// Обрабатываем команды на основе типа команды
	switch cmd := command.Command.(type) {
	// Универсальные CRUD команды
	case *api.CommandRequest_Create:
		result, err := dataService.Create(contextWithTimeout, cmd.Create)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Entity{
					Entity: result,
				},
			}
		}
		
	case *api.CommandRequest_Get:
		result, err := dataService.Get(contextWithTimeout, cmd.Get)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Entity{
					Entity: result,
				},
			}
		}
		
	case *api.CommandRequest_Update:
		result, err := dataService.Update(contextWithTimeout, cmd.Update)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Entity{
					Entity: result,
				},
			}
		}
		
	case *api.CommandRequest_Delete:
		result, err := dataService.Delete(contextWithTimeout, cmd.Delete)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Delete{
					Delete: result,
				},
			}
		}
		
	case *api.CommandRequest_List:
		result, err := dataService.List(contextWithTimeout, cmd.List)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_List{
					List: result,
				},
			}
		}
		
	case *api.CommandRequest_Search:
		result, err := dataService.Search(contextWithTimeout, cmd.Search)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_List{
					List: result,
				},
			}
		}
		
	case *api.CommandRequest_BatchCreate:
		result, err := dataService.BatchCreate(contextWithTimeout, cmd.BatchCreate)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Batch{
					Batch: result,
				},
			}
		}
		
	case *api.CommandRequest_BatchUpdate:
		result, err := dataService.BatchUpdate(contextWithTimeout, cmd.BatchUpdate)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Batch{
					Batch: result,
				},
			}
		}
		
	// Специализированные команды
	case *api.CommandRequest_GetOrganization:
		result, err := dataService.GetOrganization(contextWithTimeout, cmd.GetOrganization)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Organization{
					Organization: result,
				},
			}
		}
		
	case *api.CommandRequest_ValidateInvite:
		result, err := dataService.ValidateInvite(contextWithTimeout, cmd.ValidateInvite)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Invite{
					Invite: result,
				},
			}
		}
		
	case *api.CommandRequest_UseInvite:
		result, err := dataService.UseInvite(contextWithTimeout, cmd.UseInvite)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_User{
					User: result,
				},
			}
		}
		
	case *api.CommandRequest_SubmitForm:
		result, err := dataService.SubmitForm(contextWithTimeout, cmd.SubmitForm)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Form{
					Form: result,
				},
			}
		}
		
	case *api.CommandRequest_ListOrganizations:
		result, err := dataService.ListOrganizations(contextWithTimeout, cmd.ListOrganizations)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Organizations{
					Organizations: result,
				},
			}
		}

	case *api.CommandRequest_CreateInvite:
		result, err := dataService.CreateInvite(contextWithTimeout, cmd.CreateInvite)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Invite{
					Invite: result,
				},
			}
		}
	
	case *api.CommandRequest_GetFinancialData:
		result, err := dataService.GetFinancialData(contextWithTimeout, cmd.GetFinancialData)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_FinancialData{
					FinancialData: result,
				},
			}
		}
	
	case *api.CommandRequest_GetStaffData:
		result, err := dataService.GetStaffData(contextWithTimeout, cmd.GetStaffData)
		if err != nil {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: err.Error(),
					},
				},
			}
		} else {
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_StaffData{
					StaffData: result,
				},
			}
		}
		
	// Системные команды
	case *api.CommandRequest_SystemCommand:
		systemCommand := cmd.SystemCommand
		switch systemCommand {
		case "cache_clear":
			// Очистка кэша (заглушка)
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_System{
					System: &api.SystemResponse{
						Success: true,
						Message: "Cache cleared successfully",
						Data:    make(map[string]string),
					},
				},
			}
		case "health_check":
			// Проверка здоровья (заглушка)
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_System{
					System: &api.SystemResponse{
						Success: true,
						Message: "Service is healthy",
						Data:    make(map[string]string),
					},
				},
			}
		default:
			response = &api.CommandResponse{
				RequestId: command.RequestId,
				Response: &api.CommandResponse_Error{
					Error: &api.ErrorResponse{
						Message: "Unsupported system command: " + systemCommand,
					},
				},
			}
		}
		
	default:
		response = &api.CommandResponse{
			RequestId: command.RequestId,
			Response: &api.CommandResponse_Error{
				Error: &api.ErrorResponse{
					Message: "Unsupported command type",
				},
			},
		}
	}
	
	// Отправляем ответ обратно на сервер
	if err := stream.Send(response); err != nil {
		log.Printf("Failed to send response for request %s: %v", command.RequestId, err)
	} else {
		log.Printf("✅ Successfully processed request %s", command.RequestId)
	}
}