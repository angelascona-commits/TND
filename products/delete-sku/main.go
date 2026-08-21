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

type DeleteSKURequest struct {
	ProductID  string `json:"productId"`
	VarianteID string `json:"varianteId"`
}

func jsonResponse(statusCode int, body interface{}) (events.APIGatewayProxyResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "DELETE,OPTIONS",
			},
			Body: `{"error": "Error interno al serializar respuesta JSON"}`,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "DELETE,OPTIONS",
		},
		Body: string(jsonBody),
	}, nil
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en DeleteProductSKU: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawProdID := req.PathParameters["id"]
	rawSKUID := req.PathParameters["skuId"]

	if rawProdID == "" || rawSKUID == "" {
		var reqBody DeleteSKURequest
		if err := json.Unmarshal([]byte(req.Body), &reqBody); err == nil {
			if rawProdID == "" {
				rawProdID = reqBody.ProductID
			}
			if rawSKUID == "" {
				rawSKUID = reqBody.VarianteID
			}
		}
	}

	cleanProdID := strings.TrimPrefix(strings.TrimSpace(rawProdID), "PROD#")
	cleanSKUID := strings.TrimPrefix(strings.TrimSpace(rawSKUID), "SKU#")

	if cleanProdID == "" || cleanSKUID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Los parámetros 'id' (productId) y 'skuId' son obligatorios"})
	}

	pkVal := "PROD#" + cleanProdID
	skVal := "SKU#" + cleanSKUID

	_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pkVal},
			"SK": &types.AttributeValueMemberS{Value: skVal},
		},
	})
	if err != nil {
		log.Printf("Error al eliminar variante SKU (PK=%s, SK=%s): %v", pkVal, skVal, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al eliminar la variante SKU en DynamoDB"})
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"message":    "Variante SKU eliminada exitosamente",
		"productId":  cleanProdID,
		"varianteId": cleanSKUID,
	})
}

func main() {
	lambda.Start(handler)
}
