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
	tableName = os.Getenv("MAIN_TABLE")
	if tableName == "" {
		tableName = os.Getenv("PRODUCTS_TABLE")
	}
}

type ProductDetailResponse struct {
	ProductID   string                   `json:"productId"`
	Nombre      string                   `json:"nombre"`
	Descripcion string                   `json:"descripcion,omitempty"`
	Estado      string                   `json:"estado"`
	CategoryID  string                   `json:"categoryId"`
	BrandID     string                   `json:"brandId"`
	Variantes   []map[string]interface{} `json:"variantes"`
	Seriales    []map[string]interface{} `json:"seriales"`
}

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

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en GetProduct: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawID, ok := req.PathParameters["id"]
	if !ok || rawID == "" {
		rawID = req.QueryStringParameters["id"]
	}
	if rawID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'id' del producto es obligatorio"})
	}

	cleanID := strings.TrimPrefix(strings.TrimSpace(rawID), "PROD#")
	prodPK := "PROD#" + cleanID

	output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pkVal"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pkVal": &types.AttributeValueMemberS{Value: prodPK},
		},
	})
	if err != nil {
		log.Printf("Error al consultar producto por PK=%s: %v", prodPK, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar el producto"})
	}

	if len(output.Items) == 0 {
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "Producto no encontrado"})
	}

	var response ProductDetailResponse
	response.ProductID = cleanID
	response.Variantes = []map[string]interface{}{}
	response.Seriales = []map[string]interface{}{}

	for _, itemMap := range output.Items {
		var raw map[string]interface{}
		if err := attributevalue.UnmarshalMap(itemMap, &raw); err != nil {
			continue
		}

		sk, ok := raw["SK"].(string)
		if !ok {
			continue
		}

		if sk == "INFO_BASE" {
			if n, ok := raw["nombre"].(string); ok {
				response.Nombre = n
			}
			if d, ok := raw["descripcion"].(string); ok {
				response.Descripcion = d
			}
			if e, ok := raw["estado"].(string); ok {
				response.Estado = e
			}
			if c, ok := raw["categoryId"].(string); ok {
				response.CategoryID = c
			}
			if b, ok := raw["brandId"].(string); ok {
				response.BrandID = b
			}
		} else if strings.HasPrefix(sk, "SKU#") {
			response.Variantes = append(response.Variantes, raw)
		} else if strings.HasPrefix(sk, "SN#") {
			response.Seriales = append(response.Seriales, raw)
		}
	}

	return jsonResponse(http.StatusOK, response)
}

func main() {
	lambda.Start(handler)
}
