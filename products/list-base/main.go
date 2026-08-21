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

type ProductSearchResult struct {
	PK          string `json:"pk" dynamodbav:"PK"`
	SK          string `json:"sk" dynamodbav:"SK"`
	GSI2_PK     string `json:"gsi2Pk" dynamodbav:"GSI2_PK"`
	GSI2_SK     string `json:"gsi2Sk" dynamodbav:"GSI2_SK"`
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
	log.Printf("Petición recibida en ListProducts: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	categoryParam := req.QueryStringParameters["category"]
	if categoryParam == "" {
		categoryParam = req.QueryStringParameters["cat"]
	}
	brandParam := req.QueryStringParameters["brand"]

	cleanCat := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(categoryParam)), "CAT#")
	cleanBrand := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(brandParam)), "BRAND#")

	if cleanCat == "" && cleanBrand == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{
			"error": "Debe seleccionar al menos una Categoría o una Marca para realizar la búsqueda.",
		})
	}

	var results []ProductSearchResult

	if cleanCat != "" {
		gsi2PK := "CAT#" + cleanCat
		keyCondExpr := "GSI2_PK = :gsi2PK"
		exprValues := map[string]types.AttributeValue{
			":gsi2PK": &types.AttributeValueMemberS{Value: gsi2PK},
		}

		if cleanBrand != "" {
			gsi2SK := "BRAND#" + cleanBrand
			keyCondExpr += " AND GSI2_SK = :gsi2SK"
			exprValues[":gsi2SK"] = &types.AttributeValueMemberS{Value: gsi2SK}
		}

		output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(tableName),
			IndexName:                 aws.String("GSI2"),
			KeyConditionExpression:    aws.String(keyCondExpr),
			ExpressionAttributeValues: exprValues,
		})
		if err == nil {
			attributevalue.UnmarshalListOfMaps(output.Items, &results)
		}
	} else if cleanBrand != "" {
		categoriesToSearch := []string{"LAPTOPS", "SMARTPHONES", "TABLETS", "ACCESORIOS", "MONITORES", "GENERAL"}

		for _, c := range categoriesToSearch {
			gsi2PK := "CAT#" + c
			gsi2SK := "BRAND#" + cleanBrand

			output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
				TableName:                 aws.String(tableName),
				IndexName:                 aws.String("GSI2"),
				KeyConditionExpression:    aws.String("GSI2_PK = :gsi2PK AND GSI2_SK = :gsi2SK"),
				ExpressionAttributeValues: map[string]types.AttributeValue{
					":gsi2PK": &types.AttributeValueMemberS{Value: gsi2PK},
					":gsi2SK": &types.AttributeValueMemberS{Value: gsi2SK},
				},
			})
			if err == nil && len(output.Items) > 0 {
				var catResults []ProductSearchResult
				if attributevalue.UnmarshalListOfMaps(output.Items, &catResults) == nil {
					results = append(results, catResults...)
				}
			}
		}
	}

	if results == nil {
		results = []ProductSearchResult{}
	}

	return jsonResponse(http.StatusOK, results)
}

func main() {
	lambda.Start(handler)
}
