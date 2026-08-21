package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
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
	PK        string                 `json:"pk,omitempty" dynamodbav:"PK"`
	SK        string                 `json:"sk,omitempty" dynamodbav:"SK"`
	Category  string                 `json:"category" dynamodbav:"category,omitempty"`
	ItemID    string                 `json:"itemId" dynamodbav:"itemId"`
	Label     string                 `json:"label,omitempty" dynamodbav:"label,omitempty"`
	IsActive  *bool                  `json:"isActive,omitempty" dynamodbav:"isActive,omitempty"`
	SortOrder *int                   `json:"sortOrder,omitempty" dynamodbav:"sortOrder,omitempty"`
	Metadata  map[string]interface{} `json:"metadata,omitempty" dynamodbav:"metadata,omitempty"`
	ParentID  *string                `json:"parentId,omitempty" dynamodbav:"parentId,omitempty"`
}

func jsonResponse(statusCode int, body interface{}) (events.APIGatewayProxyResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "PATCH,OPTIONS",
			},
			Body: `{"error": "Error interno al serializar respuesta JSON"}`,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "PATCH,PUT,OPTIONS",
		},
		Body: string(jsonBody),
	}, nil
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en UpdateCatalog: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var item CatalogItem
	if err := json.Unmarshal([]byte(req.Body), &item); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	category := strings.TrimSpace(item.Category)
	if category == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'category' es obligatorio"})
	}

	itemId := strings.TrimSpace(item.ItemID)
	if itemId == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'itemId' es obligatorio"})
	}

	cleanCategory := strings.TrimPrefix(strings.ToUpper(category), "CATALOG#")
	cleanItemID := strings.TrimPrefix(strings.ToUpper(itemId), "ITEM#")

	pkVal := "CATALOG#" + cleanCategory
	skVal := "ITEM#" + cleanItemID

	updateParts := []string{}
	exprValues := map[string]types.AttributeValue{}
	exprNames := map[string]string{}

	if strings.TrimSpace(item.Label) != "" {
		updateParts = append(updateParts, "#lblAttr = :labelVal")
		exprNames["#lblAttr"] = "label"
		exprValues[":labelVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(item.Label)}
	}

	if item.IsActive != nil {
		updateParts = append(updateParts, "isActive = :activeVal")
		exprValues[":activeVal"] = &types.AttributeValueMemberBOOL{Value: *item.IsActive}
	}

	if item.SortOrder != nil {
		updateParts = append(updateParts, "sortOrder = :sortVal")
		exprValues[":sortVal"] = &types.AttributeValueMemberN{Value: strconv.Itoa(*item.SortOrder)}
	}

	if item.Metadata != nil {
		metaAv, err := attributevalue.Marshal(item.Metadata)
		if err == nil {
			updateParts = append(updateParts, "metadata = :metaVal")
			exprValues[":metaVal"] = metaAv
		}
	}

	if item.ParentID != nil {
		updateParts = append(updateParts, "parentId = :parentVal")
		exprValues[":parentVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(*item.ParentID)}
	}

	if len(updateParts) == 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "No se proporcionaron atributos válidos para actualizar"})
	}

	updateExpression := "SET " + strings.Join(updateParts, ", ")

	input := &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pkVal},
			"SK": &types.AttributeValueMemberS{Value: skVal},
		},
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: exprValues,
		ReturnValues:              types.ReturnValueUpdatedNew,
	}

	if len(exprNames) > 0 {
		input.ExpressionAttributeNames = exprNames
	}

	output, err := dynamoClient.UpdateItem(ctx, input)
	if err != nil {
		log.Printf("Error al actualizar ítem de catálogo (PK=%s, SK=%s): %v", pkVal, skVal, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al actualizar el ítem en DynamoDB mediante UpdateItem"})
	}

	var updatedAttributes map[string]interface{}
	attributevalue.UnmarshalMap(output.Attributes, &updatedAttributes)

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"message":           "Ítem de catálogo actualizado exitosamente",
		"category":          cleanCategory,
		"itemId":            cleanItemID,
		"updatedAttributes": updatedAttributes,
	})
}

func main() {
	lambda.Start(handler)
}
