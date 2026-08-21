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
	log.Printf("Petición recibida en ListProductSKUs: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawID, ok := req.PathParameters["id"]
	if !ok || rawID == "" {
		rawID = req.QueryStringParameters["productId"]
	}
	if rawID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'id' (productId) es obligatorio"})
	}

	cleanProdID := strings.TrimPrefix(strings.TrimSpace(rawID), "PROD#")
	prodPK := "PROD#" + cleanProdID

	output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pkVal AND begins_with(SK, :skPrefix)"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pkVal":    &types.AttributeValueMemberS{Value: prodPK},
			":skPrefix": &types.AttributeValueMemberS{Value: "SKU#"},
		},
	})
	if err != nil {
		log.Printf("Error al consultar variantes SKU de producto %s: %v", prodPK, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar variantes SKU en la base de datos"})
	}

	var skus []ProductSKUItem
	if err := attributevalue.UnmarshalListOfMaps(output.Items, &skus); err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar las variantes SKU"})
	}

	if skus == nil {
		skus = []ProductSKUItem{}
	}

	return jsonResponse(http.StatusOK, skus)
}

func main() {
	lambda.Start(handler)
}
