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
	PK        string `json:"pk,omitempty" dynamodbav:"PK"`
	SK        string `json:"sk,omitempty" dynamodbav:"SK"`
	GSI2_PK   string `json:"gsi2Pk,omitempty" dynamodbav:"GSI2_PK,omitempty"`
	GSI2_SK   string `json:"gsi2Sk,omitempty" dynamodbav:"GSI2_SK,omitempty"`
	DniRuc    string `json:"dniRuc" dynamodbav:"dniRuc"`
	Nombre    string `json:"nombre" dynamodbav:"nombre"`
	Correo    string `json:"correo,omitempty" dynamodbav:"correo,omitempty"`
	Direccion string `json:"direccion,omitempty" dynamodbav:"direccion,omitempty"`
	Telefono  string `json:"telefono,omitempty" dynamodbav:"telefono,omitempty"`
	Zona      string `json:"zona,omitempty" dynamodbav:"zona,omitempty"`
	Rol       string `json:"rol" dynamodbav:"rol"`
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
	log.Printf("Petición recibida en ListEntities: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	rolParam := req.QueryStringParameters["rol"]
	if rolParam == "" {
		rolParam = req.QueryStringParameters["type"]
	}
	zonaParam := req.QueryStringParameters["zona"]
	if zonaParam == "" {
		zonaParam = req.QueryStringParameters["zone"]
	}

	cleanRol := strings.TrimSpace(rolParam)
	cleanZona := strings.ToUpper(strings.TrimSpace(zonaParam))

	if cleanRol == "" && cleanZona == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{
			"error": "El parámetro 'rol' (ej: ?rol=Cliente) o 'zona' (ej: ?zona=MIRAFLORES) es obligatorio.",
		})
	}

	rolesToQuery := []string{}
	if cleanRol != "" {
		lowerRol := strings.ToLower(cleanRol)
		if strings.HasPrefix(lowerRol, "proveedor") || strings.EqualFold(cleanRol, "SUPPLIER") {
			rolesToQuery = append(rolesToQuery, "Proveedor")
		} else if strings.HasPrefix(lowerRol, "cliente") || strings.EqualFold(cleanRol, "CLIENT") {
			rolesToQuery = append(rolesToQuery, "Cliente")
		} else {
			rolesToQuery = append(rolesToQuery, cleanRol)
		}
	} else {
		rolesToQuery = append(rolesToQuery, "Cliente", "Proveedor")
	}

	var allEntities []EntityProfile

	for _, r := range rolesToQuery {
		gsi2PK := "ROL#" + r
		keyCondExpr := "GSI2_PK = :gsi2PK"
		exprValues := map[string]types.AttributeValue{
			":gsi2PK": &types.AttributeValueMemberS{Value: gsi2PK},
		}

		if cleanZona != "" {
			keyCondExpr += " AND begins_with(GSI2_SK, :gsi2SKPrefix)"
			exprValues[":gsi2SKPrefix"] = &types.AttributeValueMemberS{Value: "ZONE#" + cleanZona}
		}

		output, err := dynamoClient.Query(ctx, &dynamodb.QueryInput{
			TableName:                 aws.String(tableName),
			IndexName:                 aws.String("GSI2"),
			KeyConditionExpression:    aws.String(keyCondExpr),
			ExpressionAttributeValues: exprValues,
		})
		if err != nil {
			log.Printf("Error al consultar entidades en GSI2 (PK=%s): %v", gsi2PK, err)
			continue
		}

		var entities []EntityProfile
		if err := attributevalue.UnmarshalListOfMaps(output.Items, &entities); err == nil {
			allEntities = append(allEntities, entities...)
		}
	}

	if allEntities == nil {
		allEntities = []EntityProfile{}
	}

	return jsonResponse(http.StatusOK, allEntities)
}

func main() {
	lambda.Start(handler)
}
