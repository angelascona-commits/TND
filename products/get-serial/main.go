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

type ProductSerialItem struct {
	PK           string `json:"pk,omitempty" dynamodbav:"PK"`
	SK           string `json:"sk,omitempty" dynamodbav:"SK"`
	GSI1_PK      string `json:"gsi1Pk,omitempty" dynamodbav:"GSI1_PK,omitempty"`
	ProductID    string `json:"productId" dynamodbav:"productId"`
	NumeroSerie  string `json:"numeroSerie" dynamodbav:"numeroSerie"`
	SKUAsociado  string `json:"skuAsociado" dynamodbav:"skuAsociado"`
	EstadoFisico string `json:"estadoFisico" dynamodbav:"estadoFisico"`
	Ubicacion    string `json:"ubicacion,omitempty" dynamodbav:"ubicacion,omitempty"`
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
	log.Printf("Petición recibida en GetProductSerial: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawSN, ok := req.PathParameters["sn"]
	if !ok || rawSN == "" {
		rawSN = req.QueryStringParameters["sn"]
	}
	if rawSN == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'sn' (número de serie) es obligatorio"})
	}

	cleanSN := strings.TrimPrefix(strings.TrimSpace(rawSN), "SN#")
	gsi1PK := "SN#" + cleanSN

	output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("GSI1_PK = :snPK"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":snPK": &types.AttributeValueMemberS{Value: gsi1PK},
		},
	})
	if err != nil {
		log.Printf("Error al consultar GSI1 para rastreo de serie %s: %v", gsi1PK, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar el número de serie"})
	}

	if len(output.Items) == 0 {
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "Número de serie no encontrado"})
	}

	var serialItems []ProductSerialItem
	if err := attributevalue.UnmarshalListOfMaps(output.Items, &serialItems); err != nil {
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar el número de serie"})
	}

	return jsonResponse(http.StatusOK, serialItems[0])
}

func main() {
	lambda.Start(handler)
}
