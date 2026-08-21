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

type OrderDetailResponse struct {
	TrxID         string                   `json:"trxId"`
	UserDniRuc    string                   `json:"userDniRuc"`
	TipoOperacion string                   `json:"tipoOperacion"`
	Total         float64                  `json:"total"`
	EstadoPago    string                   `json:"estadoPago"`
	Fecha         string                   `json:"fecha"`
	Detalles      []map[string]interface{} `json:"detalles"`
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
	log.Printf("Petición recibida en GetOrder: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawID, ok := req.PathParameters["id"]
	if !ok || rawID == "" {
		rawID = req.QueryStringParameters["id"]
	}
	if rawID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'id' (trxId) es obligatorio"})
	}

	cleanID := strings.TrimPrefix(strings.TrimSpace(rawID), "TRX#")
	trxPK := "TRX#" + cleanID

	output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pkVal"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pkVal": &types.AttributeValueMemberS{Value: trxPK},
		},
	})
	if err != nil {
		log.Printf("Error al consultar transacción por PK=%s: %v", trxPK, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al consultar la transacción"})
	}

	if len(output.Items) == 0 {
		return jsonResponse(http.StatusNotFound, map[string]string{"error": "Transacción no encontrada"})
	}

	var response OrderDetailResponse
	response.TrxID = cleanID
	response.Detalles = []map[string]interface{}{}

	for _, itemMap := range output.Items {
		var raw map[string]interface{}
		if err := attributevalue.UnmarshalMap(itemMap, &raw); err != nil {
			continue
		}

		sk, ok := raw["SK"].(string)
		if !ok {
			continue
		}

		if sk == "HEADER" {
			if u, ok := raw["userDniRuc"].(string); ok {
				response.UserDniRuc = u
			}
			if t, ok := raw["tipoOperacion"].(string); ok {
				response.TipoOperacion = t
			}
			if tot, ok := raw["total"].(float64); ok {
				response.Total = tot
			}
			if est, ok := raw["estadoPago"].(string); ok {
				response.EstadoPago = est
			}
			if f, ok := raw["fecha"].(string); ok {
				response.Fecha = f
			}
		} else if strings.HasPrefix(sk, "DETAIL#") {
			response.Detalles = append(response.Detalles, raw)
		}
	}

	return jsonResponse(http.StatusOK, response)
}

func main() {
	lambda.Start(handler)
}
