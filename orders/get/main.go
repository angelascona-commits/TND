package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var dynamoClient *dynamodb.Client
var tableName string

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Error al cargar la configuración de AWS SDK: %v", err)
	}
	dynamoClient = dynamodb.NewFromConfig(cfg)
	tableName = os.Getenv("ORDERS_TABLE")
}

// Order representa la estructura de un Pedido
type Order struct {
	ID          string  `json:"id" dynamodbav:"id"`
	Customer    string  `json:"customer" dynamodbav:"customer"`
	ProductID   string  `json:"productId" dynamodbav:"productId"`
	Quantity    int     `json:"quantity" dynamodbav:"quantity"`
	TotalAmount float64 `json:"totalAmount" dynamodbav:"totalAmount"`
	Status      string  `json:"status" dynamodbav:"status"`
}

// jsonResponse construye respuestas HTTP en JSON con cabeceras CORS
func jsonResponse(statusCode int, body interface{}) (events.APIGatewayProxyResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "GET,OPTIONS",
			},
			Body: `{"error": "Error interno al serializar respuesta JSON"}`,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "GET,OPTIONS",
		},
		Body: string(jsonBody),
	}, nil
}

// handler maneja exclusivamente GET /orders/{id}
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en GetOrder: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	id, ok := req.PathParameters["id"]
	if !ok || id == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'id' en la ruta es requerido"})
	}

	output, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		log.Printf("Error al consultar DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar la base de datos"})
	}

	if output.Item == nil {
		return jsonResponse(http.StatusNotFound, map[string]string{"message": "Pedido no encontrado"})
	}

	var order Order
	if err := attributevalue.UnmarshalMap(output.Item, &order); err != nil {
		log.Printf("Error al deserializar pedido: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar los datos del pedido"})
	}

	return jsonResponse(http.StatusOK, order)
}

func main() {
	lambda.Start(handler)
}
