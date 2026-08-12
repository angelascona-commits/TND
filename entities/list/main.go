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
)

var dynamoClient *dynamodb.Client
var tableName string

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Error al cargar la configuración de AWS SDK: %v", err)
	}
	dynamoClient = dynamodb.NewFromConfig(cfg)
	tableName = os.Getenv("ENTITIES_TABLE")
}

type Entity struct {
	ID        string `json:"id" dynamodbav:"id"`
	Type      string `json:"type" dynamodbav:"type"`
	Name      string `json:"name" dynamodbav:"name"`
	Email     string `json:"email" dynamodbav:"email"`
	Phone     string `json:"phone" dynamodbav:"phone"`
	CreatedAt string `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt string `json:"updatedAt" dynamodbav:"updatedAt"`
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

// handler maneja GET /entities
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en ListEntities: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	entityType := req.QueryStringParameters["type"]

	output, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
		TableName: aws.String(tableName),
	})
	if err != nil {
		log.Printf("Error al realizar Scan en EntitiesTable: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar la tabla de entidades"})
	}

	var allEntities []Entity
	if err := attributevalue.UnmarshalListOfMaps(output.Items, &allEntities); err != nil {
		log.Printf("Error al deserializar entidades: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar la lista de entidades"})
	}

	var result []Entity
	if entityType != "" {
		for _, e := range allEntities {
			if e.Type == entityType {
				result = append(result, e)
			}
		}
	} else {
		result = allEntities
	}

	if result == nil {
		result = []Entity{}
	}

	return jsonResponse(http.StatusOK, result)
}

func main() {
	lambda.Start(handler)
}
