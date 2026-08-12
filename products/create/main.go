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
	tableName = os.Getenv("PRODUCTS_TABLE")
}

type Product struct {
	ID         string  `json:"id,omitempty" dynamodbav:"id"`
	Name       string  `json:"name" dynamodbav:"name"`
	Price      float64 `json:"price" dynamodbav:"price"`
	Category   string  `json:"category,omitempty" dynamodbav:"category,omitempty"`
	StockCount int     `json:"stockCount" dynamodbav:"stockCount"`
	Location   string  `json:"location,omitempty" dynamodbav:"location,omitempty"`
	CreatedAt  string  `json:"createdAt,omitempty" dynamodbav:"createdAt"`
	UpdatedAt  string  `json:"updatedAt,omitempty" dynamodbav:"updatedAt"`
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

// handler maneja POST /products
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en CreateProduct: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var prod Product
	if err := json.Unmarshal([]byte(req.Body), &prod); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	if prod.Name == "" || prod.Price <= 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Los campos 'name' y 'price' (>0) son obligatorios"})
	}

	now := time.Now().UTC().Format(time.RFC3339)
	prod.ID = uuid.New().String()
	prod.CreatedAt = now
	prod.UpdatedAt = now

	av, err := attributevalue.MarshalMap(prod)
	if err != nil {
		log.Printf("Error al serializar producto para DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al preparar los datos del producto"})
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		log.Printf("Error al guardar producto en DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al guardar el producto en la base de datos"})
	}

	return jsonResponse(http.StatusCreated, prod)
}

func main() {
	lambda.Start(handler)
}
