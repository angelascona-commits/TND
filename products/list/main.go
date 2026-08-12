package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
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

// Product representa la entidad unificada de Producto e Inventario
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

// handler maneja GET /products permitiendo buscar/filtrar por ?category=... o ?search=...
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en ListProducts: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	categoryFilter := req.QueryStringParameters["category"]
	searchQuery := strings.ToLower(req.QueryStringParameters["search"])

	output, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		log.Printf("Error al realizar Scan en ProductsTable: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar los productos"})
	}

	var allProducts []Product
	if err := attributevalue.UnmarshalListOfMaps(output.Items, &allProducts); err != nil {
		log.Printf("Error al deserializar lista de productos: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar los productos"})
	}

	var filtered []Product
	for _, p := range allProducts {
		matchCategory := categoryFilter == "" || strings.EqualFold(p.Category, categoryFilter)
		matchSearch := searchQuery == "" || strings.Contains(strings.ToLower(p.Name), searchQuery) || strings.Contains(strings.ToLower(p.Category), searchQuery)

		if matchCategory && matchSearch {
			filtered = append(filtered, p)
		}
	}

	if filtered == nil {
		filtered = []Product{}
	}

	return jsonResponse(http.StatusOK, filtered)
}

func main() {
	lambda.Start(handler)
}
