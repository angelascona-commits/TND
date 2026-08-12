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

// Order representa la transacción de Venta o Compra
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

// handler maneja GET /orders/search (Lambda dedicada para búsquedas por rango de fechas, producto o tipo de transacción)
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en SearchOrders (Lambda Dedicada): %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	orderType := req.QueryStringParameters["type"]
	productID := req.QueryStringParameters["productId"]
	startDateStr := req.QueryStringParameters["startDate"]
	endDateStr := req.QueryStringParameters["endDate"]

	var startDate, endDate time.Time
	var err error

	if startDateStr != "" {
		if len(startDateStr) == 10 {
			startDateStr += "T00:00:00Z"
		}
		startDate, _ = time.Parse(time.RFC3339, startDateStr)
	}

	if endDateStr != "" {
		if len(endDateStr) == 10 {
			endDateStr += "T23:59:59Z"
		}
		endDate, _ = time.Parse(time.RFC3339, endDateStr)
	}

	output, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		log.Printf("Error al realizar Scan en OrdersTable: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al buscar transacciones"})
	}

	var allOrders []Order
	if err := attributevalue.UnmarshalListOfMaps(output.Items, &allOrders); err != nil {
		log.Printf("Error al deserializar órdenes: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar los resultados"})
	}

	var results []Order
	for _, o := range allOrders {
		matchType := orderType == "" || o.Type == orderType
		matchProduct := productID == "" || o.ProductID == productID

		matchDate := true
		if o.CreatedAt != "" {
			t, err := time.Parse(time.RFC3339, o.CreatedAt)
			if err == nil {
				if !startDate.IsZero() && t.Before(startDate) {
					matchDate = false
				}
				if !endDate.IsZero() && t.After(endDate) {
					matchDate = false
				}
			}
		}

		if matchType && matchProduct && matchDate {
			results = append(results, o)
		}
	}

	if results == nil {
		results = []Order{}
	}

	return jsonResponse(http.StatusOK, results)
}

func main() {
	lambda.Start(handler)
}
