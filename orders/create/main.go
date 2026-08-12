package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/google/uuid"
)

var dynamoClient *dynamodb.Client
var ordersTableName string
var productsTableName string

func init() {
	cfg, err := config.LoadDefaultConfig(context.TODO())
	if err != nil {
		log.Fatalf("Error al cargar la configuración de AWS SDK: %v", err)
	}
	dynamoClient = dynamodb.NewFromConfig(cfg)
	ordersTableName = os.Getenv("ORDERS_TABLE")
	productsTableName = os.Getenv("PRODUCTS_TABLE")
}

type Order struct {
	ID          string  `json:"id,omitempty" dynamodbav:"id"`
	Type        string  `json:"type" dynamodbav:"type"`
	EntityID    string  `json:"entityId,omitempty" dynamodbav:"entityId,omitempty"`
	Customer    string  `json:"customer,omitempty" dynamodbav:"customer,omitempty"`
	ProductID   string  `json:"productId" dynamodbav:"productId"`
	Quantity    int     `json:"quantity" dynamodbav:"quantity"`
	TotalAmount float64 `json:"totalAmount" dynamodbav:"totalAmount"`
	Status      string  `json:"status,omitempty" dynamodbav:"status,omitempty"`
	CreatedAt   string  `json:"createdAt,omitempty" dynamodbav:"createdAt"`
}

func jsonResponse(statusCode int, body interface{}) (events.APIGatewayProxyResponse, error) {
	jsonBody, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusInternalServerError,
			Headers: map[string]string{
				"Content-Type":                 "application/json",
				"Access-Control-Allow-Origin":  "*",
				"Access-Control-Allow-Methods": "POST,OPTIONS",
			},
			Body: `{"error": "Error interno al serializar respuesta JSON"}`,
		}, nil
	}

	return events.APIGatewayProxyResponse{
		StatusCode: statusCode,
		Headers: map[string]string{
			"Content-Type":                 "application/json",
			"Access-Control-Allow-Origin":  "*",
			"Access-Control-Allow-Methods": "POST,OPTIONS",
		},
		Body: string(jsonBody),
	}, nil
}

// handler maneja POST /orders
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en CreateOrder: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var order Order
	if err := json.Unmarshal([]byte(req.Body), &order); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	if order.ProductID == "" || order.Quantity <= 0 || order.TotalAmount <= 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Los campos 'productId', 'quantity' (>0) y 'totalAmount' son obligatorios"})
	}

	if order.Type != "PURCHASE" {
		order.Type = "SALE"
	}

	now := time.Now().UTC().Format(time.RFC3339)
	order.ID = uuid.New().String()
	order.Status = "COMPLETED"
	order.CreatedAt = now

	av, err := attributevalue.MarshalMap(order)
	if err != nil {
		log.Printf("Error al serializar orden para DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al preparar los datos de la transacción"})
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(ordersTableName),
		Item:      av,
	})
	if err != nil {
		log.Printf("Error al guardar orden en OrdersTable: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al registrar la transacción"})
	}

	if productsTableName != "" {
		updateExpr := "SET stockCount = stockCount - :q, updatedAt = :u"
		if order.Type == "PURCHASE" {
			updateExpr = "SET stockCount = stockCount + :q, updatedAt = :u"
		}

		_, err = dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(productsTableName),
			Key: map[string]types.AttributeValue{
				"id": &types.AttributeValueMemberS{Value: order.ProductID},
			},
			UpdateExpression: aws.String(updateExpr),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":q": &types.AttributeValueMemberN{Value: strconv.Itoa(order.Quantity)},
				":u": &types.AttributeValueMemberS{Value: now},
			},
		})
		if err != nil {
			log.Printf("Advertencia: No se pudo actualizar el stock en ProductsTable: %v", err)
		}
	}

	return jsonResponse(http.StatusCreated, order)
}

func main() {
	lambda.Start(handler)
}
