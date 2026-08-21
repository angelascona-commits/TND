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
		tableName = os.Getenv("ENTITIES_TABLE")
	}
}

type DeleteEntityRequest struct {
	DniRuc string `json:"dniRuc"`
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
	log.Printf("Petición recibida en DeleteEntity: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawID, ok := req.PathParameters["id"]
	if !ok || rawID == "" {
		rawID = req.QueryStringParameters["id"]
	}
	if rawID == "" {
		var reqBody DeleteEntityRequest
		if err := json.Unmarshal([]byte(req.Body), &reqBody); err == nil {
			rawID = reqBody.DniRuc
		}
	}

	rawID = strings.TrimSpace(rawID)
	if rawID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'id' o 'dniRuc' es obligatorio para eliminar una entidad"})
	}

	cleanID := strings.TrimPrefix(strings.ToUpper(rawID), "USER#")
	pkVal := "USER#" + cleanID

	_, err := dynamoClient.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pkVal},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
	})
	if err != nil {
		log.Printf("Error al eliminar entidad PK=%s: %v", pkVal, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al eliminar la entidad en DynamoDB"})
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"message": "Entidad eliminada exitosamente",
		"dniRuc":  cleanID,
	})
}

func main() {
	lambda.Start(handler)
}
