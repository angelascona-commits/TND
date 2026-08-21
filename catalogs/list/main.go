package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sort"
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
		tableName = os.Getenv("CATALOGS_TABLE")
	}
}

type CatalogItem struct {
	PK        string                 `json:"pk" dynamodbav:"PK"`
	SK        string                 `json:"sk" dynamodbav:"SK"`
	Category  string                 `json:"category,omitempty" dynamodbav:"category,omitempty"`
	ItemID    string                 `json:"itemId,omitempty" dynamodbav:"itemId,omitempty"`
	Label     string                 `json:"label" dynamodbav:"label"`
	IsActive  bool                   `json:"isActive" dynamodbav:"isActive"`
	SortOrder int                    `json:"sortOrder" dynamodbav:"sortOrder"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" dynamodbav:"metadata,omitempty"`
	ParentID  string                 `json:"parentId,omitempty" dynamodbav:"parentId,omitempty"`
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
	log.Printf("Petición recibida en ListCatalogs: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	categoriesParam := req.QueryStringParameters["categories"]
	if categoriesParam == "" {
		categoriesParam = req.QueryStringParameters["category"]
	}
	if categoriesParam == "" {
		categoriesParam = req.QueryStringParameters["type"]
	}

	if strings.TrimSpace(categoriesParam) == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{
			"error": "El parámetro 'categories' o 'category' es obligatorio (ej: ?categories=CATEGORIAS,MARCAS).",
		})
	}

	var items []CatalogItem

	categoryList := strings.Split(categoriesParam, ",")
	for _, cat := range categoryList {
		cleanCategory := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(cat)), "CATALOG#")
		if cleanCategory == "" {
			continue
		}
		pkVal := "CATALOG#" + cleanCategory

		output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
			TableName:              aws.String(tableName),
			KeyConditionExpression: aws.String("PK = :pkVal AND begins_with(SK, :skPrefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":pkVal":    &types.AttributeValueMemberS{Value: pkVal},
				":skPrefix": &types.AttributeValueMemberS{Value: "ITEM#"},
			},
		})
		if err != nil {
			log.Printf("Error al consultar catálogos por PK=%s: %v", pkVal, err)
			continue
		}

		var catItems []CatalogItem
		if err := attributevalue.UnmarshalListOfMaps(output.Items, &catItems); err == nil {
			items = append(items, catItems...)
		}
	}

	if items == nil {
		items = []CatalogItem{}
	} else {
		sort.Slice(items, func(i, j int) bool {
			return items[i].SortOrder < items[j].SortOrder
		})
	}

	return jsonResponse(http.StatusOK, items)
}

func main() {
	lambda.Start(handler)
}
