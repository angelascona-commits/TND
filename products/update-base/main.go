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

type UpdateProductRequest struct {
	ProductID   string   `json:"productId"`
	ID          string   `json:"id,omitempty"`
	Nombre      string   `json:"nombre,omitempty"`
	Descripcion string   `json:"descripcion,omitempty"`
	Estado      string   `json:"estado,omitempty"`
	CategoryID  string   `json:"categoryId,omitempty"`
	BrandID     string   `json:"brandId,omitempty"`
	Precio      *float64 `json:"precio,omitempty"`
	StockTotal  *int     `json:"stockTotal,omitempty"`
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
	log.Printf("Petición recibida en UpdateProduct: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var reqBody UpdateProductRequest
	if err := json.Unmarshal([]byte(req.Body), &reqBody); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	prodID := strings.TrimSpace(reqBody.ProductID)
	if prodID == "" {
		prodID = strings.TrimSpace(reqBody.ID)
	}
	if prodID == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'productId' es obligatorio"})
	}

	cleanID := strings.TrimPrefix(prodID, "PROD#")
	prodPK := "PROD#" + cleanID

	infoUpdateParts := []string{}
	infoExprValues := map[string]types.AttributeValue{}

	if strings.TrimSpace(reqBody.Nombre) != "" {
		infoUpdateParts = append(infoUpdateParts, "nombre = :nombreVal")
		infoExprValues[":nombreVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(reqBody.Nombre)}
	}

	if strings.TrimSpace(reqBody.Descripcion) != "" {
		infoUpdateParts = append(infoUpdateParts, "descripcion = :descVal")
		infoExprValues[":descVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(reqBody.Descripcion)}
	}

	if strings.TrimSpace(reqBody.Estado) != "" {
		infoUpdateParts = append(infoUpdateParts, "estado = :estVal")
		infoExprValues[":estVal"] = &types.AttributeValueMemberS{Value: strings.ToUpper(strings.TrimSpace(reqBody.Estado))}
	}

	if strings.TrimSpace(reqBody.CategoryID) != "" {
		cleanCat := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(reqBody.CategoryID)), "CAT#")
		infoUpdateParts = append(infoUpdateParts, "categoryId = :catVal", "GSI2_PK = :gsi2PkVal")
		infoExprValues[":catVal"] = &types.AttributeValueMemberS{Value: cleanCat}
		infoExprValues[":gsi2PkVal"] = &types.AttributeValueMemberS{Value: "CAT#" + cleanCat}
	}

	if strings.TrimSpace(reqBody.BrandID) != "" {
		cleanBrand := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(reqBody.BrandID)), "BRAND#")
		infoUpdateParts = append(infoUpdateParts, "brandId = :brandVal", "GSI2_SK = :gsi2SkVal")
		infoExprValues[":brandVal"] = &types.AttributeValueMemberS{Value: cleanBrand}
		infoExprValues[":gsi2SkVal"] = &types.AttributeValueMemberS{Value: "BRAND#" + cleanBrand}
	}

	if len(infoUpdateParts) > 0 {
		infoUpdateExpr := "SET " + strings.Join(infoUpdateParts, ", ")
		_, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: prodPK},
				"SK": &types.AttributeValueMemberS{Value: "INFO_BASE"},
			},
			UpdateExpression:          aws.String(infoUpdateExpr),
			ExpressionAttributeValues: infoExprValues,
		})
		if err != nil {
			log.Printf("Error al actualizar INFO_BASE del producto %s: %v", prodPK, err)
		}
	}

	skuUpdateParts := []string{}
	skuExprValues := map[string]types.AttributeValue{}

	if reqBody.Precio != nil {
		skuUpdateParts = append(skuUpdateParts, "precio = :precioVal")
		skuExprValues[":precioVal"] = &types.AttributeValueMemberN{Value: strconv.FormatFloat(*reqBody.Precio, 'f', 2, 64)}
	}

	if reqBody.StockTotal != nil {
		skuUpdateParts = append(skuUpdateParts, "stockTotal = :stockVal")
		skuExprValues[":stockVal"] = &types.AttributeValueMemberN{Value: strconv.Itoa(*reqBody.StockTotal)}
	}

	if len(skuUpdateParts) > 0 {
		skuUpdateExpr := "SET " + strings.Join(skuUpdateParts, ", ")
		_, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: prodPK},
				"SK": &types.AttributeValueMemberS{Value: "SKU#V1"},
			},
			UpdateExpression:          aws.String(skuUpdateExpr),
			ExpressionAttributeValues: skuExprValues,
		})
		if err != nil {
			log.Printf("Error al actualizar SKU#V1 del producto %s: %v", prodPK, err)
		}
	}

	return jsonResponse(http.StatusOK, map[string]string{
		"message":   "Producto actualizado exitosamente con UpdateItem",
		"productId": cleanID,
	})
}

func main() {
	lambda.Start(handler)
}
