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
	tableName = os.Getenv("MAIN_TABLE")
	if tableName == "" {
		tableName = os.Getenv("PRODUCTS_TABLE")
	}
}

type ProductSerialItem struct {
	PK           string `json:"pk,omitempty" dynamodbav:"PK"`
	SK           string `json:"sk,omitempty" dynamodbav:"SK"`
	GSI1_PK      string `json:"gsi1Pk,omitempty" dynamodbav:"GSI1_PK,omitempty"`
	GSI1_SK      string `json:"gsi1Sk,omitempty" dynamodbav:"GSI1_SK,omitempty"`
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
				"Access-Control-Allow-Methods": "POST,OPTIONS",
			},
			Body: `{"error": "Error interno al serializar respuesta JSON"}`,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST,OPTIONS",
		},
		Body: string(jsonBody),
	}, nil
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en CreateProductSerial: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawID, ok := req.PathParameters["id"]
	if !ok || rawID == "" {
		rawID = req.QueryStringParameters["id"]
	}

	var item ProductSerialItem
	if err := json.Unmarshal([]byte(req.Body), &item); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	prodID := strings.TrimSpace(item.ProductID)
	if prodID == "" {
		prodID = strings.TrimSpace(rawID)
	}
	if prodID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'productId' o 'id' es obligatorio"})
	}

	cleanProdID := strings.TrimPrefix(prodID, "PROD#")

	sn := strings.TrimPrefix(strings.TrimSpace(item.NumeroSerie), "SN#")
	if sn == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'numeroSerie' es obligatorio"})
	}

	skuID := strings.TrimPrefix(strings.TrimSpace(item.SKUAsociado), "SKU#")
	if skuID == "" {
		skuID = "V1"
	}

	estadoFisico := strings.TrimSpace(item.EstadoFisico)
	if estadoFisico == "" {
		estadoFisico = "Disponible"
	}

	item.PK = "PROD#" + cleanProdID
	item.SK = "SN#" + sn
	item.GSI1_PK = "SN#" + sn
	item.GSI1_SK = "INFO"
	item.ProductID = cleanProdID
	item.NumeroSerie = sn
	item.SKUAsociado = skuID
	item.EstadoFisico = estadoFisico

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		log.Printf("Error al serializar unidad serializada para DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al preparar los datos del serial"})
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		log.Printf("Error al guardar unidad serializada en DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al registrar la unidad serializada en la base de datos"})
	}

	return jsonResponse(http.StatusCreated, item)
}

func main() {
	lambda.Start(handler)
}
