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

type UpdateSerialRequest struct {
	ProductID    string `json:"productId"`
	NumeroSerie  string `json:"numeroSerie"`
	EstadoFisico string `json:"estadoFisico,omitempty"`
	Ubicacion    string `json:"ubicacion,omitempty"`
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
	log.Printf("Petición recibida en UpdateProductSerial: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawProdID := req.PathParameters["id"]
	rawSN := req.PathParameters["sn"]

	var reqBody UpdateSerialRequest
	if err := json.Unmarshal([]byte(req.Body), &reqBody); err == nil {
		if rawProdID == "" {
			rawProdID = reqBody.ProductID
		}
		if rawSN == "" {
			rawSN = reqBody.NumeroSerie
		}
	}

	cleanProdID := strings.TrimPrefix(strings.TrimSpace(rawProdID), "PROD#")
	cleanSN := strings.TrimPrefix(strings.TrimSpace(rawSN), "SN#")

	if cleanProdID == "" || cleanSN == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Los parámetros 'productId' y 'numeroSerie' son obligatorios"})
	}

	pkVal := "PROD#" + cleanProdID
	skVal := "SN#" + cleanSN

	updateParts := []string{}
	exprValues := map[string]types.AttributeValue{}

	if strings.TrimSpace(reqBody.EstadoFisico) != "" {
		updateParts = append(updateParts, "estadoFisico = :estVal")
		exprValues[":estVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(reqBody.EstadoFisico)}
	}

	if strings.TrimSpace(reqBody.Ubicacion) != "" {
		updateParts = append(updateParts, "ubicacion = :ubVal")
		exprValues[":ubVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(reqBody.Ubicacion)}
	}

	if len(updateParts) == 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "No se proporcionaron campos válidos para actualizar en la unidad serializada"})
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
		log.Printf("Error al actualizar unidad serializada (PK=%s, SK=%s): %v", pkVal, skVal, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al actualizar la unidad serializada en DynamoDB mediante UpdateItem"})
	}

	var updatedAttributes map[string]interface{}
	attributevalue.UnmarshalMap(output.Attributes, &updatedAttributes)

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"message":           "Unidad serializada actualizada exitosamente",
		"productId":         cleanProdID,
		"numeroSerie":       cleanSN,
		"updatedAttributes": updatedAttributes,
	})
}

func main() {
	lambda.Start(handler)
}
