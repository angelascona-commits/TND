package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

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
	tableName = os.Getenv("PRODUCTS_TABLE")
}

// Product define el modelo de Producto en Go y DynamoDB
type Product struct {
	ID    string  `json:"id" dynamodbav:"id"`
	Name  string  `json:"name" dynamodbav:"name"`
	Price float64 `json:"price" dynamodbav:"price"`
}

// jsonResponse construye respuestas HTTP en JSON con cabeceras CORS
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

// handler maneja exclusivamente GET /products/{id}
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en GetProduct: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	id, ok := req.PathParameters["id"]
	if !ok || id == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'id' en la ruta es requerido"})
	}

	output, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"id": &types.AttributeValueMemberS{Value: id},
		},
	})
	if err != nil {
		log.Printf("Error al consultar DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar la base de datos"})
	}

	if output.Item == nil {
		return jsonResponse(http.StatusNotFound, map[string]string{"message": "Producto no encontrado"})
	}

	var product Product
	if err := attributevalue.UnmarshalMap(output.Item, &product); err != nil {
		log.Printf("Error al deserializar producto: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar los datos del producto"})
	}

	return jsonResponse(http.StatusOK, product)
}

func main() {
	lambda.Start(handler)
}
