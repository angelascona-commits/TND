package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/google/uuid"
)

var dynamoClient *dynamodb.Client
var tableName string

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Error al cargar la configuración de AWS SDK: %v", err)
	}
	dynamoClient = dynamodb.NewFromConfig(cfg)
	tableName = os.Getenv("ENTITIES_TABLE")
}

type Entity struct {
	ID        string `json:"id,omitempty" dynamodbav:"id"`
	Type      string `json:"type" dynamodbav:"type"`
	Name      string `json:"name" dynamodbav:"name"`
	Email     string `json:"email,omitempty" dynamodbav:"email,omitempty"`
	Phone     string `json:"phone,omitempty" dynamodbav:"phone,omitempty"`
	CreatedAt string `json:"createdAt,omitempty" dynamodbav:"createdAt"`
	UpdatedAt string `json:"updatedAt,omitempty" dynamodbav:"updatedAt"`
}

func jsonResponse(statusCode int, body interface{}) (events.APIGatewayProxyResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST,OPTIONS",
			},
			Body: `{"error": "Error interno al serializar respuesta JSON"}`,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST,OPTIONS",
		},
		Body: string(jsonBody),
	}, nil
}

// handler maneja POST /entities
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en CreateEntity: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var entity Entity
	if err := json.Unmarshal([]byte(req.Body), &entity); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	if entity.Name == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'name' es obligatorio"})
	}

	if entity.Type != "CUSTOMER" && entity.Type != "SUPPLIER" {
		entity.Type = "CUSTOMER"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	entity.ID = uuid.New().String()
	entity.CreatedAt = now
	entity.UpdatedAt = now

	av, err := attributevalue.MarshalMap(entity)
	if err != nil {
		log.Printf("Error al serializar entidad para DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al preparar los datos de la entidad"})
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		log.Printf("Error al guardar entidad en DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al registrar la entidad en la base de datos"})
	}

	return jsonResponse(http.StatusCreated, entity)
}

func main() {
	lambda.Start(handler)
}
