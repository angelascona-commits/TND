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
		tableName = os.Getenv("PRODUCTS_TABLE")
	}
}

type UpdateSKURequest struct {
	ProductID  string   `json:"productId"`
	VarianteID string   `json:"varianteId"`
	Precio     *float64 `json:"precio,omitempty"`
	StockTotal *int     `json:"stockTotal,omitempty"`
	Color      string   `json:"color,omitempty"`
	RAM        string   `json:"ram,omitempty"`
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
	log.Printf("Petición recibida en UpdateProductSKU: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawProdID := req.PathParameters["id"]
	rawSKUID := req.PathParameters["skuId"]

	var reqBody UpdateSKURequest
	if err := json.Unmarshal([]byte(req.Body), &reqBody); err == nil {
		if rawProdID == "" {
			rawProdID = reqBody.ProductID
		}
		if rawSKUID == "" {
			rawSKUID = reqBody.VarianteID
		}
	}

	cleanProdID := strings.TrimPrefix(strings.TrimSpace(rawProdID), "PROD#")
	cleanSKUID := strings.TrimPrefix(strings.TrimSpace(rawSKUID), "SKU#")

	if cleanProdID == "" || cleanSKUID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Los parámetros 'productId' y 'varianteId' son obligatorios"})
	}

	pkVal := "PROD#" + cleanProdID
	skVal := "SKU#" + cleanSKUID

	updateParts := []string{}
	exprValues := map[string]types.AttributeValue{}

	if reqBody.Precio != nil {
		updateParts = append(updateParts, "precio = :precioVal")
		exprValues[":precioVal"] = &types.AttributeValueMemberN{Value: strconv.FormatFloat(*reqBody.Precio, 'f', 2, 64)}
	}

	if reqBody.StockTotal != nil {
		updateParts = append(updateParts, "stockTotal = :stockVal")
		exprValues[":stockVal"] = &types.AttributeValueMemberN{Value: strconv.Itoa(*reqBody.StockTotal)}
	}

	if strings.TrimSpace(reqBody.Color) != "" {
		updateParts = append(updateParts, "color = :colorVal")
		exprValues[":colorVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(reqBody.Color)}
	}

	if strings.TrimSpace(reqBody.RAM) != "" {
		updateParts = append(updateParts, "ram = :ramVal")
		exprValues[":ramVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(reqBody.RAM)}
	}

	if len(updateParts) == 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "No se proporcionaron campos válidos para actualizar en la variante SKU"})
	}

	updateExpression := "SET " + strings.Join(updateParts, ", ")

	output, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pkVal},
			"SK": &types.AttributeValueMemberS{Value: skVal},
		},
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: exprValues,
		ReturnValues:              types.ReturnValueUpdatedNew,
	})
	if err != nil {
		log.Printf("Error al actualizar variante SKU (PK=%s, SK=%s): %v", pkVal, skVal, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al actualizar la variante SKU en DynamoDB mediante UpdateItem"})
	}

	var updatedAttributes map[string]interface{}
	attributevalue.UnmarshalMap(output.Attributes, &updatedAttributes)

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"message":           "Variante SKU actualizada exitosamente",
		"productId":         cleanProdID,
		"varianteId":        cleanSKUID,
		"updatedAttributes": updatedAttributes,
	})
}

func main() {
	lambda.Start(handler)
}
