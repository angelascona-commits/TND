package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"

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
	tableName = os.Getenv("PRODUCTS_TABLE")
}

// Product representa el registro unificado de Producto e Inventario
type Product struct {
	ID         string                 `json:"id" dynamodbav:"id"`
	Name       string                 `json:"name" dynamodbav:"name"`
	Price      float64                `json:"price" dynamodbav:"price"`
	Category   string                 `json:"category" dynamodbav:"category"`
	StockCount int                    `json:"stockCount" dynamodbav:"stockCount"`
	Location   string                 `json:"location" dynamodbav:"location"`
	Attributes map[string]interface{} `json:"attributes" dynamodbav:"attributes"`
	CreatedAt  string                 `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt  string                 `json:"updatedAt" dynamodbav:"updatedAt"`
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

// handler maneja GET /products/search (Lambda dedicada para búsquedas y filtros por categoría/precio/características)
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en SearchProducts (Lambda Dedicada): %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	categoryParam := req.QueryStringParameters["category"]
	queryParam := strings.ToLower(req.QueryStringParameters["query"])
	minPriceStr := req.QueryStringParameters["minPrice"]
	maxPriceStr := req.QueryStringParameters["maxPrice"]

	var minPrice, maxPrice float64
	if minPriceStr != "" {
		minPrice, _ = strconv.ParseFloat(minPriceStr, 64)
	}
	if maxPriceStr != "" {
		maxPrice, _ = strconv.ParseFloat(maxPriceStr, 64)
	}

	output, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		log.Printf("Error al realizar Scan en ProductsTable: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al buscar productos"})
	}

	var allProducts []Product
	if err := attributevalue.UnmarshalListOfMaps(output.Items, &allProducts); err != nil {
		log.Printf("Error al deserializar productos: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar los productos"})
	}

	var results []Product
	for _, p := range allProducts {
		matchCategory := categoryParam == "" || strings.EqualFold(p.Category, categoryParam)
		matchQuery := queryParam == "" || strings.Contains(strings.ToLower(p.Name), queryParam) || strings.Contains(strings.ToLower(p.Category), queryParam)
		matchMinPrice := minPrice == 0 || p.Price >= minPrice
		matchMaxPrice := maxPrice == 0 || p.Price <= maxPrice

		if matchCategory && matchQuery && matchMinPrice && matchMaxPrice {
			results = append(results, p)
		}
	}

	if results == nil {
		results = []Product{}
	}

	return jsonResponse(http.StatusOK, results)
}

func main() {
	lambda.Start(handler)
}
