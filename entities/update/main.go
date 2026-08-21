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
		tableName = os.Getenv("ENTITIES_TABLE")
	}
}

type EntityProfile struct {
	DniRuc    string `json:"dniRuc"`
	Nombre    string `json:"nombre,omitempty"`
	Correo    string `json:"correo,omitempty"`
	Direccion string `json:"direccion,omitempty"`
	Telefono  string `json:"telefono,omitempty"`
	Zona      string `json:"zona,omitempty"`
	Rol       string `json:"rol,omitempty"`
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
			"Access-Control-Allow-Methods": "PATCH,OPTIONS",
		},
		Body: string(jsonBody),
	}, nil
}

func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	log.Printf("Petición recibida en UpdateEntity: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var entity EntityProfile
	if err := json.Unmarshal([]byte(req.Body), &entity); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	dniRuc := strings.TrimSpace(entity.DniRuc)
	if dniRuc == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'dniRuc' es obligatorio para actualizar la entidad"})
	}

	cleanID := strings.TrimPrefix(strings.ToUpper(dniRuc), "USER#")
	pkVal := "USER#" + cleanID

	updateParts := []string{}
	exprValues := map[string]types.AttributeValue{}

	if strings.TrimSpace(entity.Nombre) != "" {
		updateParts = append(updateParts, "nombre = :nombreVal")
		exprValues[":nombreVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(entity.Nombre)}
	}

	if strings.TrimSpace(entity.Correo) != "" {
		updateParts = append(updateParts, "correo = :correoVal")
		exprValues[":correoVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(entity.Correo)}
	}

	if strings.TrimSpace(entity.Direccion) != "" {
		updateParts = append(updateParts, "direccion = :direccionVal")
		exprValues[":direccionVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(entity.Direccion)}
	}

	if strings.TrimSpace(entity.Telefono) != "" {
		updateParts = append(updateParts, "telefono = :telefonoVal")
		exprValues[":telefonoVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(entity.Telefono)}
	}

	var newRol string
	if strings.TrimSpace(entity.Rol) != "" {
		rol := strings.TrimSpace(entity.Rol)
		if strings.EqualFold(rol, "Proveedor") || strings.EqualFold(rol, "SUPPLIER") {
			newRol = "Proveedor"
		} else {
			newRol = "Cliente"
		}
		updateParts = append(updateParts, "rol = :rolVal", "GSI2_PK = :gsi2PkVal")
		exprValues[":rolVal"] = &types.AttributeValueMemberS{Value: newRol}
		exprValues[":gsi2PkVal"] = &types.AttributeValueMemberS{Value: "ROL#" + newRol}
	}

	if strings.TrimSpace(entity.Zona) != "" {
		cleanZona := strings.ToUpper(strings.TrimSpace(entity.Zona))
		updateParts = append(updateParts, "zona = :zonaVal", "GSI2_SK = :gsi2SkVal")
		exprValues[":zonaVal"] = &types.AttributeValueMemberS{Value: strings.TrimSpace(entity.Zona)}
		exprValues[":gsi2SkVal"] = &types.AttributeValueMemberS{Value: "ZONE#" + cleanZona + "#USER#" + cleanID}
	}

	if len(updateParts) == 0 {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "No se proporcionaron atributos válidos para actualizar"})
	}

	updateExpression := "SET " + strings.Join(updateParts, ", ")

	output, err := dynamoClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: pkVal},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
		UpdateExpression:          aws.String(updateExpression),
		ExpressionAttributeValues: exprValues,
		ReturnValues:              types.ReturnValueUpdatedNew,
	})
	if err != nil {
		log.Printf("Error al actualizar entidad PK=%s: %v", pkVal, err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al actualizar la entidad en DynamoDB mediante UpdateItem"})
	}

	var updatedAttributes map[string]interface{}
	attributevalue.UnmarshalMap(output.Attributes, &updatedAttributes)

	return jsonResponse(http.StatusOK, map[string]interface{}{
		"message":           "Entidad actualizada exitosamente",
		"dniRuc":            cleanID,
		"updatedAttributes": updatedAttributes,
	})
}

func main() {
	lambda.Start(handler)
}
