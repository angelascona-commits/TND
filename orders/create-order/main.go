package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
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

type OrderDetailItem struct {
	PK             string  `json:"pk" dynamodbav:"PK"`
	SK             string  `json:"sk" dynamodbav:"SK"`
	TrxID          string  `json:"trxId" dynamodbav:"trxId"`
	ProductID      string  `json:"productId" dynamodbav:"productId"`
	SKUID          string  `json:"skuId" dynamodbav:"skuId"`
	SerialNumber   string  `json:"serialNumber,omitempty" dynamodbav:"serialNumber,omitempty"`
	Cantidad       int     `json:"cantidad" dynamodbav:"cantidad"`
	PrecioUnitario float64 `json:"precioUnitario" dynamodbav:"precioUnitario"`
	Subtotal       float64 `json:"subtotal" dynamodbav:"subtotal"`
}

type CreateOrderItemRequest struct {
	ProductID      string  `json:"productId"`
	SKUID          string  `json:"skuId"`
	SerialNumber   string  `json:"serialNumber,omitempty"`
	NumeroSerie    string  `json:"numeroSerie,omitempty"`
	Cantidad       int     `json:"cantidad"`
	PrecioUnitario float64 `json:"precioUnitario"`
}

type CreateOrderRequest struct {
	TrxID         string                   `json:"trxId,omitempty"`
	UserDniRuc    string                   `json:"userDniRuc"`
	TipoOperacion string                   `json:"tipoOperacion"`
	EstadoPago    string                   `json:"estadoPago,omitempty"`
	Items         []CreateOrderItemRequest `json:"items"`
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

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en CreateOrder: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var reqBody CreateOrderRequest
	if err := json.Unmarshal([]byte(req.Body), &reqBody); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	userDniRuc := strings.TrimPrefix(strings.ToUpper(strings.TrimSpace(reqBody.UserDniRuc)), "USER#")
	if userDniRuc == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'userDniRuc' es obligatorio"})
	}

	tipoOp := strings.TrimSpace(reqBody.TipoOperacion)
	if strings.EqualFold(tipoOp, "Compra") || strings.EqualFold(tipoOp, "BUY") || strings.EqualFold(tipoOp, "PURCHASE") {
		tipoOp = "Compra"
	} else {
		tipoOp = "Venta"
	}

	if len(reqBody.Items) == 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "La orden debe contener al menos un ítem de detalle"})
	}

	trxID := strings.TrimSpace(reqBody.TrxID)
	if trxID == "" {
		trxID = uuid.New().String()
	} else {
		trxID = strings.TrimPrefix(trxID, "TRX#")
	}

	fechaIso := time.Now().UTC().Format(time.RFC3339)
	estadoPago := strings.TrimSpace(reqBody.EstadoPago)
	if estadoPago == "" {
		estadoPago = "COMPLETADO"
	}

	var totalOrder float64 = 0
	transactItems := []types.TransactWriteItem{}

	for idx, itemReq := range reqBody.Items {
		prodID := strings.TrimPrefix(strings.TrimSpace(itemReq.ProductID), "PROD#")
		if prodID == "" {
			prodID = fmt.Sprintf("PROD_GEN_%d", idx+1)
		}
		skuID := strings.TrimPrefix(strings.TrimSpace(itemReq.SKUID), "SKU#")
		if skuID == "" {
			skuID = "V1"
		}

		sn := strings.TrimSpace(itemReq.SerialNumber)
		if sn == "" {
			sn = strings.TrimSpace(itemReq.NumeroSerie)
		}
		sn = strings.TrimPrefix(sn, "SN#")

		cant := itemReq.Cantidad
		if sn != "" {
			cant = 1
		}
		if cant <= 0 {
			cant = 1
		}

		subtotal := float64(cant) * itemReq.PrecioUnitario
		totalOrder += subtotal

		var detailSK string
		if sn != "" {
			detailSK = fmt.Sprintf("DETAIL#SKU#%s#SN#%s", skuID, sn)
		} else {
			detailSK = fmt.Sprintf("DETAIL#SKU#%s", skuID)
		}

		detailItem := OrderDetailItem{
			PK:             "TRX#" + trxID,
			SK:             detailSK,
			TrxID:          trxID,
			ProductID:      prodID,
			SKUID:          skuID,
			SerialNumber:   sn,
			Cantidad:       cant,
			PrecioUnitario: itemReq.PrecioUnitario,
			Subtotal:       subtotal,
		}

		detailAv, err := attributevalue.MarshalMap(detailItem)
		if err != nil {
			log.Printf("Error al serializar detalle %s: %v", detailSK, err)
			return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al procesar el detalle de la orden"})
		}

		transactItems = append(transactItems, types.TransactWriteItem{
			Put: &types.Put{
				TableName: aws.String(tableName),
				Item:      detailAv,
			},
		})

		if sn != "" {
			estadoFisico := "Vendido"
			if tipoOp == "Compra" {
				estadoFisico = "Disponible"
			}

			serialKey := map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: "PROD#" + prodID},
				"SK": &types.AttributeValueMemberS{Value: "SN#" + sn},
			}

			transactItems = append(transactItems, types.TransactWriteItem{
				Update: &types.Update{
					TableName:        aws.String(tableName),
					Key:              serialKey,
					UpdateExpression: aws.String("SET estadoFisico = :estVal, GSI1_PK = :gsi1Val, GSI1_SK = :gsi1SkVal, productId = :pVal, skuAsociado = :skuVal"),
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":estVal":     &types.AttributeValueMemberS{Value: estadoFisico},
						":gsi1Val":    &types.AttributeValueMemberS{Value: "SN#" + sn},
						":gsi1SkVal":  &types.AttributeValueMemberS{Value: "INFO"},
						":pVal":       &types.AttributeValueMemberS{Value: prodID},
						":skuVal":     &types.AttributeValueMemberS{Value: skuID},
					},
				},
			})
		}
	}

	headerItem := OrderHeaderItem{
		PK:            "TRX#" + trxID,
		SK:            "HEADER",
		GSI1_PK:       "USER#" + userDniRuc,
		GSI1_SK:       fechaIso,
		GSI2_PK:       "TYPE#" + tipoOp,
		GSI2_SK:       fechaIso,
		TrxID:         trxID,
		UserDniRuc:    userDniRuc,
		TipoOperacion: tipoOp,
		Total:         totalOrder,
		EstadoPago:    estadoPago,
		Fecha:         fechaIso,
	}

	headerAv, err := attributevalue.MarshalMap(headerItem)
	if err != nil {
		log.Printf("Error al serializar cabecera de la transacción: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al preparar la cabecera de la transacción"})
	}

	transactItems = append([]types.TransactWriteItem{
		{
			Put: &types.Put{
				TableName: aws.String(tableName),
				Item:      headerAv,
			},
		},
	}, transactItems...)

	_, err = dynamoClient.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{
		TransactItems: transactItems,
	})
	if err != nil {
		log.Printf("Error al ejecutar TransactWriteItems para la orden %s: %v", trxID, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al registrar la transacción atómica en DynamoDB"})
	}

	return jsonResponse(http.StatusCreated, map[string]interface{}{
		"header": headerItem,
		"items":  reqBody.Items,
	})
}

func main() {
	lambda.Start(handler)
}
