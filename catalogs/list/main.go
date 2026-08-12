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
	tableName = os.Getenv("CATALOGS_TABLE")
}

type CatalogItem struct {
	PK          string `json:"pk" dynamodbav:"PK"`
	SK          string `json:"sk" dynamodbav:"SK"`
	Name        string `json:"name" dynamodbav:"name"`
	Description string `json:"description,omitempty" dynamodbav:"description,omitempty"`
	CreatedAt   string `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt   string `json:"updatedAt" dynamodbav:"updatedAt"`
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

// handler maneja GET /catalogs
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en ListCatalogs: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	catalogType := req.QueryStringParameters["type"]

	var catalogItems []CatalogItem

	if catalogType != "" {
		output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			KeyConditionExpression: aws.String("PK = :pkVal"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pkVal": &types.AttributeValueMemberS{Value: catalogType},
			},
		})
		if err != nil {
			log.Printf("Error al consultar catálogos por PK=%s: %v", catalogType, err)
			return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar el catálogo"})
		}
		if err := attributevalue.UnmarshalListOfMaps(output.Items, &catalogItems); err != nil {
			log.Printf("Error al deserializar catálogo: %v", err)
			return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar el catálogo"})
		}
	} else {
		output, err := dynamoClient.Scan(ctx, &dynamodb.ScanInput{
			TableName: aws.String(tableName),
		})
		if err != nil {
			log.Printf("Error al realizar Scan en CatalogsTable: %v", err)
			return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al listar la tabla de catálogos"})
		}
		if err := attributevalue.UnmarshalListOfMaps(output.Items, &catalogItems); err != nil {
			log.Printf("Error al deserializar catálogo completo: %v", err)
			return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar los catálogos"})
		}
	}

	if catalogItems == nil {
		catalogItems = []CatalogItem{}
	}

	return jsonResponse(http.StatusOK, catalogItems)
}

func main() {
	lambda.Start(handler)
}
