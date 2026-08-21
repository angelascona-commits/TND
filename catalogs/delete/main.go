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
		tableName = os.Getenv("CATALOGS_TABLE")
	}
}

type DeleteCatalogRequest struct {
	Category string `json:"category"`
	ItemID   string `json:"itemId"`
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
	log.Printf("Petición recibida en DeleteCatalog: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	category := req.QueryStringParameters["category"]
	itemId := req.QueryStringParameters["itemId"]

	if category == "" || itemId == "" {
		var reqBody DeleteCatalogRequest
		if err := json.Unmarshal([]byte(req.Body), &reqBody); err == nil {
			if category == "" {
				category = reqBody.Category
			}
			if itemId == "" {
				itemId = reqBody.ItemID
			}
		}
	}

	category = strings.TrimSpace(category)
	itemId = strings.TrimSpace(itemId)

	if category == "" || itemId == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{
			"error": "Los parámetros 'category' e 'itemId' son obligatorios para eliminar un ítem del catálogo",
		})
	}

	cleanCategory := strings.TrimPrefix(strings.ToUpper(category), "CATALOG#")
	cleanItemID := strings.TrimPrefix(strings.ToUpper(itemId), "ITEM#")

	pkVal := "CATALOG#" + cleanCategory
	skVal := "ITEM#" + cleanItemID

	_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pkVal},
			"SK": &types.AttributeValueMemberS{Value: skVal},
		},
	})
	if err != nil {
		log.Printf("Error al eliminar ítem de catálogo (PK=%s, SK=%s): %v", pkVal, skVal, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al eliminar el ítem en DynamoDB"})
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"message":  "Ítem de catálogo eliminado exitosamente",
		"category": cleanCategory,
		"itemId":   cleanItemID,
	})
}

func main() {
	lambda.Start(handler)
}
