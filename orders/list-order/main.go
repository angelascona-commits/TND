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
		tableName = os.Getenv("ORDERS_TABLE")
	}
}

type OrderHeaderItem struct {
	PK            string  `json:"pk" dynamodbav:"PK"`
	SK            string  `json:"sk" dynamodbav:"SK"`
	GSI1_PK       string  `json:"gsi1Pk" dynamodbav:"GSI1_PK"`
	GSI1_SK       string  `json:"gsi1Sk" dynamodbav:"GSI1_SK"`
	GSI2_PK       string  `json:"gsi2Pk,omitempty" dynamodbav:"GSI2_PK,omitempty"`
	GSI2_SK       string  `json:"gsi2Sk,omitempty" dynamodbav:"GSI2_SK,omitempty"`
	TrxID         string  `json:"trxId" dynamodbav:"trxId"`
	UserDniRuc    string  `json:"userDniRuc" dynamodbav:"userDniRuc"`
	TipoOperacion string  `json:"tipoOperacion" dynamodbav:"tipoOperacion"`
	Total         float64 `json:"total" dynamodbav:"total"`
	EstadoPago    string  `json:"estadoPago" dynamodbav:"estadoPago"`
	Fecha         string  `json:"fecha" dynamodbav:"fecha"`
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
	log.Printf("Petición recibida en ListOrders: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	userParam := req.QueryStringParameters["user"]
	if userParam == "" {
		userParam = req.QueryStringParameters["dniRuc"]
	}
	tipoParam := req.QueryStringParameters["tipo"]
	if tipoParam == "" {
		tipoParam = req.QueryStringParameters["type"]
	}
	startDate := req.QueryStringParameters["startDate"]
	if startDate == "" {
		startDate = req.QueryStringParameters["from"]
	}
	endDate := req.QueryStringParameters["endDate"]
	if endDate == "" {
		endDate = req.QueryStringParameters["to"]
	}

	cleanUser := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(userParam)), "USER#")
	cleanTipo := strings.TrimSpace(tipoParam)
	if strings.EqualFold(cleanTipo, "Compra") || strings.EqualFold(cleanTipo, "Compras") {
		cleanTipo = "Compra"
	} else if strings.EqualFold(cleanTipo, "Venta") || strings.EqualFold(cleanTipo, "Ventas") {
		cleanTipo = "Venta"
	}

	startPrefix := strings.TrimSpace(startDate)
	endPrefix := strings.TrimSpace(endDate)
	if endPrefix != "" && len(endPrefix) == 10 {
		endPrefix += "T23:59:59Z"
	}
	if startPrefix != "" && len(startPrefix) == 10 {
		startPrefix += "T00:00:00Z"
	}

	var orders []OrderHeaderItem

	if cleanUser != "" {
		gsi1PK := "USER#" + cleanUser
		keyCondExpr := "GSI1_PK = :gsi1PK"
		exprValues := map[string]types.AttributeValue{
			":gsi1PK": &types.AttributeValueMemberS{Value: gsi1PK},
		}

		if startPrefix != "" && endPrefix != "" {
			keyCondExpr += " AND GSI1_SK BETWEEN :startDate AND :endDate"
			exprValues[":startDate"] = &types.AttributeValueMemberS{Value: startPrefix}
			exprValues[":endDate"] = &types.AttributeValueMemberS{Value: endPrefix}
		} else if startPrefix != "" {
			keyCondExpr += " AND GSI1_SK >= :startDate"
			exprValues[":startDate"] = &types.AttributeValueMemberS{Value: startPrefix}
		} else if endPrefix != "" {
			keyCondExpr += " AND GSI1_SK <= :endDate"
			exprValues[":endDate"] = &types.AttributeValueMemberS{Value: endPrefix}
		}

		output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(tableName),
			IndexName:                 aws.String("GSI1"),
			KeyConditionExpression:    aws.String(keyCondExpr),
			ExpressionAttributeValues: exprValues,
			ScanIndexForward:          aws.Bool(false),
		})
		if err == nil {
			attributevalue.UnmarshalListOfMaps(output.Items, &orders)
		}
	} else {
		tiposToQuery := []string{}
		if cleanTipo != "" {
			tiposToQuery = append(tiposToQuery, cleanTipo)
		} else {
			tiposToQuery = append(tiposToQuery, "Venta", "Compra")
		}

		for _, t := range tiposToQuery {
			gsi2PK := "TYPE#" + t
			keyCondExpr := "GSI2_PK = :gsi2PK"
			exprValues := map[string]types.AttributeValue{
				":gsi2PK": &types.AttributeValueMemberS{Value: gsi2PK},
			}

			if startPrefix != "" && endPrefix != "" {
				keyCondExpr += " AND GSI2_SK BETWEEN :startDate AND :endDate"
				exprValues[":startDate"] = &types.AttributeValueMemberS{Value: startPrefix}
				exprValues[":endDate"] = &types.AttributeValueMemberS{Value: endPrefix}
			} else if startPrefix != "" {
				keyCondExpr += " AND GSI2_SK >= :startDate"
				exprValues[":startDate"] = &types.AttributeValueMemberS{Value: startPrefix}
			} else if endPrefix != "" {
				keyCondExpr += " AND GSI2_SK <= :endDate"
				exprValues[":endDate"] = &types.AttributeValueMemberS{Value: endPrefix}
			}

			output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
				TableName:                 aws.String(tableName),
				IndexName:                 aws.String("GSI2"),
				KeyConditionExpression:    aws.String(keyCondExpr),
				ExpressionAttributeValues: exprValues,
				ScanIndexForward:          aws.Bool(false),
			})
			if err == nil && len(output.Items) > 0 {
				var subOrders []OrderHeaderItem
				if attributevalue.UnmarshalListOfMaps(output.Items, &subOrders) == nil {
					orders = append(orders, subOrders...)
				}
			}
		}
	}

	if orders == nil {
		orders = []OrderHeaderItem{}
	} else {
		sort.Slice(orders, func(i, j int) bool {
			return orders[i].Fecha > orders[j].Fecha
		})
	}

	return jsonResponse(http.StatusOK, orders)
}

func main() {
	lambda.Start(handler)
}
