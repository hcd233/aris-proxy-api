package enum

type JSONSchemaType = string

const (
	JSONSchemaTypeString  JSONSchemaType = "string"
	JSONSchemaTypeNumber  JSONSchemaType = "number"
	JSONSchemaTypeBoolean JSONSchemaType = "boolean"
	JSONSchemaTypeArray   JSONSchemaType = "array"
	JSONSchemaTypeObject  JSONSchemaType = "object"
)
