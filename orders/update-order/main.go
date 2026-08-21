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
		tableName = os.Getenv("ORDERS_TABLE")
	}
}

type UpdateOrderRequest struct {
	TrxID      string `json:"trxId"`
	ID         string `json:"id,omitempty"`
	EstadoPago string `json:"estadoPago,omitempty"`
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
	log.Printf("Petición recibida en UpdateOrder: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var reqBody UpdateOrderRequest
	if err := json.Unmarshal([]byte(req.Body), &reqBody); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	trxID := strings.TrimSpace(reqBody.TrxID)
	if trxID == "" {
		trxID = strings.TrimSpace(reqBody.ID)
	}
	if trxID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'trxId' es obligatorio"})
	}

	cleanID := strings.TrimPrefix(trxID, "TRX#")
	trxPK := "TRX#" + cleanID

	updateParts := []string{}
	exprValues := map[string]types.AttributeValue{}

	if strings.TrimSpace(reqBody.EstadoPago) != "" {
		updateParts = append(updateParts, "estadoPago = :estVal")
		exprValues[":estVal"] = &types.AttributeValueMemberS{Value: strings.ToUpper(strings.TrimSpace(reqBody.EstadoPago))}
	}

	if len(updateParts) == 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "No se proporcionaron atributos válidos para actualizar la transacción"})
	}

	updateExpression := "SET " + strings.Join(updateParts, ", ")

	output, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: trxPK},
			"SK": &types.AttributeValueMemberS{Value: "HEADER"},
		},
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: exprValues,
		ReturnValues:              types.ReturnValueUpdatedNew,
	})
	if err != nil {
		log.Printf("Error al actualizar la transacción PK=%s: %v", trxPK, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al actualizar la transacción en DynamoDB mediante UpdateItem"})
	}

	var updatedAttributes map[string]interface{}
	attributevalue.UnmarshalMap(output.Attributes, &updatedAttributes)

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"message":           "Transacción actualizada exitosamente",
		"trxId":             cleanID,
		"updatedAttributes": updatedAttributes,
	})
}

func main() {
	lambda.Start(handler)
}
