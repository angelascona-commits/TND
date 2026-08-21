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
	"github.com/google/uuid"
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

type ProductBaseItem struct {
	PK          string `json:"pk,omitempty" dynamodbav:"PK"`
	SK          string `json:"sk,omitempty" dynamodbav:"SK"`
	GSI2_PK     string `json:"gsi2Pk,omitempty" dynamodbav:"GSI2_PK,omitempty"`
	GSI2_SK     string `json:"gsi2Sk,omitempty" dynamodbav:"GSI2_SK,omitempty"`
	ProductID   string `json:"productId" dynamodbav:"productId"`
	Nombre      string `json:"nombre" dynamodbav:"nombre"`
	Descripcion string `json:"descripcion,omitempty" dynamodbav:"descripcion,omitempty"`
	Estado      string `json:"estado" dynamodbav:"estado"`
	CategoryID  string `json:"categoryId" dynamodbav:"categoryId"`
	BrandID     string `json:"brandId" dynamodbav:"brandId"`
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
	log.Printf("Petición recibida en CreateProduct (Base): %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var item ProductBaseItem
	if err := json.Unmarshal([]byte(req.Body), &item); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	nombre := strings.TrimSpace(item.Nombre)
	if nombre == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'nombre' es obligatorio"})
	}

	catID := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(item.CategoryID)), "CAT#")
	if catID == "" {
		catID = "GENERAL"
	}

	brandID := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(item.BrandID)), "BRAND#")
	if brandID == "" {
		brandID = "GENERICA"
	}

	prodID := strings.TrimSpace(item.ProductID)
	if prodID == "" {
		prodID = uuid.New().String()
	} else {
		prodID = strings.TrimPrefix(prodID, "PROD#")
	}

	estado := strings.ToUpper(strings.TrimSpace(item.Estado))
	if estado == "" {
		estado = "ACTIVO"
	}

	item.PK = "PROD#" + prodID
	item.SK = "INFO_BASE"
	item.GSI2_PK = "CAT#" + catID
	item.GSI2_SK = "BRAND#" + brandID
	item.ProductID = prodID
	item.Nombre = nombre
	item.Estado = estado
	item.CategoryID = catID
	item.BrandID = brandID

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		log.Printf("Error al serializar producto base para DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al preparar datos del producto base"})
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		log.Printf("Error al guardar producto base en DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al guardar el producto base en DynamoDB"})
	}

	return jsonResponse(http.StatusCreated, item)
}

func main() {
	lambda.Start(handler)
}
