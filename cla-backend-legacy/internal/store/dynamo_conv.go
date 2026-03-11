package store

import (
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// ToInterface recursively converts DynamoDB AttributeValue trees into
// json.Marshal-friendly Go types.
//
// IMPORTANT: for 1:1 parity with the legacy Python (pynamodb) to_dict() behavior,
// DynamoDB numbers are represented as *strings* (not float64).
func ToInterface(av types.AttributeValue) any {
	switch v := av.(type) {
	case *types.AttributeValueMemberS:
		return v.Value
	case *types.AttributeValueMemberN:
		// pynamodb NumberAttribute.serialize() yields a string.
		return v.Value
	case *types.AttributeValueMemberBOOL:
		return v.Value
	case *types.AttributeValueMemberNULL:
		return nil
	case *types.AttributeValueMemberM:
		m := make(map[string]any, len(v.Value))
		for k, vv := range v.Value {
			m[k] = ToInterface(vv)
		}
		return m
	case *types.AttributeValueMemberL:
		out := make([]any, 0, len(v.Value))
		for _, vv := range v.Value {
			out = append(out, ToInterface(vv))
		}
		return out
	case *types.AttributeValueMemberSS:
		// pynamodb UnicodeSetAttribute serializes into a list-like JSON structure.
		out := make([]string, 0, len(v.Value))
		out = append(out, v.Value...)
		return out
	case *types.AttributeValueMemberNS:
		// Keep numeric set members as strings.
		out := make([]string, 0, len(v.Value))
		out = append(out, v.Value...)
		return out
	case *types.AttributeValueMemberBS:
		// Not expected for legacy API responses.
		out := make([][]byte, 0, len(v.Value))
		out = append(out, v.Value...)
		return out
	case *types.AttributeValueMemberB:
		return v.Value
	default:
		return nil
	}
}

// ItemToInterfaceMap converts a DynamoDB item map into a JSON-friendly map.
func ItemToInterfaceMap(item map[string]types.AttributeValue) map[string]any {
	if item == nil {
		return nil
	}
	out := make(map[string]any, len(item))
	for k, av := range item {
		out[k] = ToInterface(av)
	}
	return out
}
