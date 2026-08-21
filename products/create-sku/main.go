package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

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

type ProductSKUItem struct {
	PK         string  `json:"pk,omitempty" dynamodbav:"PK"`
	SK         string  `json:"sk,omitempty" dynamodbav:"SK"`
	ProductID  string  `json:"productId" dynamodbav:"productId"`
	VarianteID string  `json:"varianteId" dynamodbav:"varianteId"`
	Precio     float64 `json:"precio" dynamodbav:"precio"`
	StockTotal int     `json:"stockTotal" dynamodbav:"stockTotal"`
	Color      string  `json:"color,omitempty" dynamodbav:"color,omitempty"`
	RAM        string  `json:"ram,omitempty" dynamodbav:"ram,omitempty"`
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

func sanitizeString(s string) string {
	reg := regexp.MustCompile("[^a-zA-Z0-9]+")
	return strings.ToUpper(strings.Trim(reg.ReplaceAllString(s, "-"), "-"))
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en CreateProductSKU: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rawID, ok := req.PathParameters["id"]
	if !ok || rawID == "" {
		rawID = req.QueryStringParameters["id"]
	}

	var item ProductSKUItem
	if err := json.Unmarshal([]byte(req.Body), &item); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	prodID := strings.TrimSpace(item.ProductID)
	if prodID == "" {
		prodID = strings.TrimSpace(rawID)
	}
	if prodID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El parámetro 'productId' o 'id' es obligatorio"})
	}

	cleanProdID := strings.TrimPrefix(prodID, "PROD#")

	if item.Precio <= 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'precio' debe ser mayor a 0"})
	}

	cleanColor := sanitizeString(item.Color)
	cleanRAM := sanitizeString(item.RAM)

	variantID := sanitizeString(strings.TrimPrefix(strings.TrimSpace(item.VarianteID), "SKU#"))
	if variantID == "" {
		parts := []string{}

		getProdOut, err := dynamoClient.GetItem(ctx, &dynamodb.GetItemInput{
			TableName: aws.String(tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: "PROD#" + cleanProdID},
				"SK": &types.AttributeValueMemberS{Value: "INFO_BASE"},
			},
		})
		if err == nil && len(getProdOut.Item) > 0 {
			var baseProd struct {
				Nombre string `dynamodbav:"nombre"`
			}
			if attributevalue.UnmarshalMap(getProdOut.Item, &baseProd) == nil && baseProd.Nombre != "" {
				upperName := strings.ToUpper(baseProd.Nombre)
				if strings.Contains(upperName, "S24") {
					parts = append(parts, "S24")
				} else if strings.Contains(upperName, "S23") {
					parts = append(parts, "S23")
				} else if strings.Contains(upperName, "IPHONE") {
					parts = append(parts, "IPHONE")
				} else if strings.Contains(upperName, "MACBOOK") {
					parts = append(parts, "MBP")
				} else {
					words := strings.Fields(sanitizeString(baseProd.Nombre))
					if len(words) > 0 {
						parts = append(parts, words[0])
					}
				}
			}
		}

		if cleanColor != "" {
			parts = append(parts, cleanColor)
		}
		if cleanRAM != "" {
			parts = append(parts, cleanRAM)
		}

		if len(parts) > 0 {
			variantID = strings.Join(parts, "-")
		} else {
			variantID = fmt.Sprintf("SKU-%d", time.Now().Unix()%10000)
		}
	}

	item.PK = "PROD#" + cleanProdID
	item.SK = "SKU#" + variantID
	item.ProductID = cleanProdID
	item.VarianteID = variantID

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		log.Printf("Error al serializar variante SKU para DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al preparar los datos de la variante SKU"})
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		log.Printf("Error al guardar variante SKU en DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al registrar la variante SKU en la base de datos"})
	}

	return jsonResponse(http.StatusCreated, item)
}

func main() {
	lambda.Start(handler)
}
