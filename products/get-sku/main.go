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

type ProductSKUItem struct {
	PK         string  `json:"pk,omitempty" dynamodbav:"PK"`
	SK         string  `json:"sk,omitempty" dynamodbav:"SK"`
	ProductID  string  `json:"productId" dynamodbav:"productId"`
	VarianteID string  `json:"varianteId" dynamodbav:"varianteId"`
	Precio     float64 `json:"precio" dynamodbav:"precio"`
	StockTotal int     `json:"stockTotal" dynamodbav:"stockTotal"`
	Color      string  `json:"color,omitempty" dynamodbav:"color,omitempty"`
	RAM        string  `json:"ram,omitempty" dynamodbav:"ram,omitempty"`
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
	log.Printf("Petición recibida en GetProductSKU: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	prodID := req.PathParameters["id"]
	skuID := req.PathParameters["skuId"]

	if prodID == "" {
		prodID = req.QueryStringParameters["productId"]
	}
	if skuID == "" {
		skuID = req.QueryStringParameters["skuId"]
	}

	if prodID == "" || skuID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Los parámetros 'id' (productId) y 'skuId' son obligatorios"})
	}

	cleanProdID := strings.TrimPrefix(strings.TrimSpace(prodID), "PROD#")
	cleanSKUID := strings.TrimPrefix(strings.TrimSpace(skuID), "SKU#")

	pkVal := "PROD#" + cleanProdID
	skVal := "SKU#" + cleanSKUID

	output, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pkVal},
			"SK": &types.AttributeValueMemberS{Value: skVal},
		},
	})
	if err != nil {
		log.Printf("Error al consultar variante SKU (PK=%s, SK=%s): %v", pkVal, skVal, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar la variante SKU en la base de datos"})
	}

	if output.Item == nil {
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "Variante SKU no encontrada"})
	}

	var skuItem ProductSKUItem
	if err := attributevalue.UnmarshalMap(output.Item, &skuItem); err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar la variante SKU"})
	}

	return jsonResponse(http.StatusOK, skuItem)
}

func main() {
	lambda.Start(handler)
}
