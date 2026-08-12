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
)

var dynamoClient *dynamodb.Client
var ordersTableName string

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Error al cargar la configuración de AWS SDK: %v", err)
	}
	dynamoClient = dynamodb.NewFromConfig(cfg)
	ordersTableName = os.Getenv("ORDERS_TABLE")
}

// Order representa la orden de venta o compra asociada a la entidad
type Order struct {
	ID          string  `json:"id" dynamodbav:"id"`
	Type        string  `json:"type" dynamodbav:"type"`
	EntityID    string  `json:"entityId" dynamodbav:"entityId"`
	Customer    string  `json:"customer" dynamodbav:"customer"`
	ProductID   string  `json:"productId" dynamodbav:"productId"`
	Quantity    int     `json:"quantity" dynamodbav:"quantity"`
	TotalAmount float64 `json:"totalAmount" dynamodbav:"totalAmount"`
	Status      string  `json:"status" dynamodbav:"status"`
	CreatedAt   string  `json:"createdAt" dynamodbav:"createdAt"`
}

// jsonResponse construye respuestas HTTP JSON con cabeceras CORS
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

// handler maneja GET /entities/{id}/orders (Lambda dedicada para el historial de un Cliente o Proveedor)
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en GetEntityHistory: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	entityID, ok := req.PathParameters["id"]
	if !ok || entityID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'id' de la entidad es requerido"})
	}

	output, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(ordersTableName),
	})
	if err != nil {
		log.Printf("Error al consultar OrdersTable: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar la base de datos de órdenes"})
	}

	var allOrders []Order
	if err := attributevalue.UnmarshalListOfMaps(output.Items, &allOrders); err != nil {
		log.Printf("Error al deserializar órdenes: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar el historial"})
	}

	var history []Order
	for _, o := range allOrders {
		if o.EntityID == entityID || o.Customer == entityID {
			history = append(history, o)
		}
	}

	if history == nil {
		history = []Order{}
	}

	return jsonResponse(http.StatusOK, history)
}

func main() {
	lambda.Start(handler)
}
