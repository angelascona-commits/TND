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
	log.Printf("Petición recibida en SearchOrder (Garantías por SN): %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	snParam := req.QueryStringParameters["sn"]
	if snParam == "" {
		snParam = req.QueryStringParameters["serialNumber"]
	}

	if strings.TrimSpace(snParam) == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{
			"error": "El parámetro 'sn' (número de serie) es obligatorio para el rastreo de garantías. No se permiten operaciones Scan.",
		})
	}

	cleanSN := strings.TrimPrefix(strings.TrimSpace(snParam), "SN#")
	gsi1PK := "SN#" + cleanSN

	output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("GSI1_PK = :gsi1PK"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":gsi1PK": &types.AttributeValueMemberS{Value: gsi1PK},
		},
	})
	if err != nil {
		log.Printf("Error al consultar GSI1 para trazabilidad de serie %s: %v", gsi1PK, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al rastrear la garantía del número de serie"})
	}

	if len(output.Items) == 0 {
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "No se encontraron registros de garantía u orden para el número de serie ingresado"})
	}

	var results []map[string]interface{}
	if err := attributevalue.UnmarshalListOfMaps(output.Items, &results); err != nil {
		log.Printf("Error al deserializar resultados de garantía: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar la búsqueda por número de serie"})
	}

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"serialNumber": cleanSN,
		"records":      results,
	})
}

func main() {
	lambda.Start(handler)
}
