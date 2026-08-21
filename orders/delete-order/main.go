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
		tableName = os.Getenv("ORDERS_TABLE")
	}
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
	log.Printf("Petición recibida en DeleteOrder: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawID, ok := req.PathParameters["id"]
	if !ok || rawID == "" {
		rawID = req.QueryStringParameters["id"]
	}
	if rawID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'id' (trxId) es obligatorio"})
	}

	cleanID := strings.TrimPrefix(strings.TrimSpace(rawID), "TRX#")
	trxPK := "TRX#" + cleanID

	output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pkVal"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pkVal": &types.AttributeValueMemberS{Value: trxPK},
		},
	})
	if err != nil {
		log.Printf("Error al consultar ítems de la orden PK=%s para eliminar: %v", trxPK, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar los ítems de la transacción"})
	}

	if len(output.Items) == 0 {
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "Transacción no encontrada"})
	}

	var writeRequests []types.WriteRequest
	for _, item := range output.Items {
		skVal := item["SK"]
		writeRequests = append(writeRequests, types.WriteRequest{
			DeleteRequest: &types.DeleteRequest{
				Key: map[string]types.AttributeValue{
					"PK": &types.AttributeValueMemberS{Value: trxPK},
					"SK": skVal,
				},
			},
		})
	}

	_, err = dynamoClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: writeRequests,
		},
	})
	if err != nil {
		log.Printf("Error al eliminar los ítems de la orden PK=%s: %v", trxPK, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al eliminar la transacción en DynamoDB"})
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"message": "Transacción y todas sus líneas de detalle eliminadas exitosamente",
		"trxId":   cleanID,
	})
}

func main() {
	lambda.Start(handler)
}
