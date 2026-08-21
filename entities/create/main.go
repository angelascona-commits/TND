package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

var dynamoClient *dynamodb.Client
var tableName string
var digitsRegex = regexp.MustCompile(`^\d+$`)

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
	log.Printf("Petición recibida en CreateEntity: %s", req.HTTPMethod)

	if req.HTTPMethod == http.MethodOptions {
		return jsonResponse(http.StatusOK, map[string]string{"status": "ok"})
	}

	var entity EntityProfile
	if err := json.Unmarshal([]byte(req.Body), &entity); err != nil {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "Formato JSON de entrada inválido"})
	}

	rawDniRuc := strings.TrimSpace(entity.DniRuc)
	if rawDniRuc == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'dniRuc' es obligatorio"})
	}

	cleanID := strings.TrimPrefix(strings.ToUpper(rawDniRuc), "USER#")

	if !digitsRegex.MatchString(cleanID) || (len(cleanID) != 8 && len(cleanID) != 11) {
		return jsonResponse(http.StatusBadRequest, map[string]string{
			"error": "Identificador de documento inválido. El DNI debe ser numérico de 8 dígitos y el RUC numérico de 11 dígitos.",
		})
	}

	nombre := strings.TrimSpace(entity.Nombre)
	if nombre == "" {
		return jsonResponse(http.StatusBadRequest, map[string]string{"error": "El campo 'nombre' es obligatorio"})
	}

	rol := strings.TrimSpace(entity.Rol)
	if strings.EqualFold(rol, "Proveedor") || strings.EqualFold(rol, "SUPPLIER") {
		rol = "Proveedor"
	} else {
		rol = "Cliente"
	}

	cleanZona := strings.ToUpper(strings.TrimSpace(entity.Zona))

	entity.PK = "USER#" + cleanID
	entity.SK = "PROFILE"
	entity.DniRuc = cleanID
	entity.Nombre = nombre
	entity.Rol = rol
	entity.Zona = strings.TrimSpace(entity.Zona)

	entity.GSI2_PK = "ROL#" + rol
	if cleanZona != "" {
		entity.GSI2_SK = "ZONE#" + cleanZona + "#USER#" + cleanID
	} else {
		entity.GSI2_SK = "USER#" + cleanID
	}

	av, err := attributevalue.MarshalMap(entity)
	if err != nil {
		log.Printf("Error al serializar entidad para DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al preparar los datos de la entidad"})
	}

	_, err = dynamoClient.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      av,
	})
	if err != nil {
		log.Printf("Error al guardar entidad en DynamoDB: %v", err)
		return jsonResponse(http.StatusInternalServerError, map[string]string{"error": "Error al registrar la entidad en la base de datos"})
	}

	return jsonResponse(http.StatusCreated, entity)
}

func main() {
	lambda.Start(handler)
}
