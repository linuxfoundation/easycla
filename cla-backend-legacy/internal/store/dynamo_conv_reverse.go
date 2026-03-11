package store

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// InterfaceMapToItem converts a JSON-style map into a DynamoDB AttributeValue map.
//
// NOTE: This intentionally mirrors the existing ItemToInterfaceMap() behavior which
// represents DynamoDB numbers as strings for Python/pynamodb parity. When converting
// back, purely-numeric strings are stored as DynamoDB numbers (N).
//
// This conversion is used sparingly (primarily for create/update flows where we build
// maps directly). When preserving exact DynamoDB attribute types is important, prefer
// patching the existing AttributeValue map directly.
func InterfaceMapToItem(in map[string]any) (map[string]types.AttributeValue, error) {
	if in == nil {
		return map[string]types.AttributeValue{}, nil
	}
	out := make(map[string]types.AttributeValue, len(in))
	for k, v := range in {
		av, err := interfaceToAV(v)
		if err != nil {
			return nil, fmt.Errorf("InterfaceMapToItem: key %q: %w", k, err)
		}
		if av == nil {
			// nil AV means omit the attribute (used for empty sets, etc.).
			continue
		}
		out[k] = av
	}
	return out, nil
}

func interfaceToAV(v any) (types.AttributeValue, error) {
	switch vv := v.(type) {
	case nil:
		// Preserve explicit nulls.
		return &types.AttributeValueMemberNULL{Value: true}, nil
	case types.AttributeValue:
		return vv, nil
	case string:
		if isNumericString(vv) {
			return &types.AttributeValueMemberN{Value: vv}, nil
		}
		return &types.AttributeValueMemberS{Value: vv}, nil
	case bool:
		return &types.AttributeValueMemberBOOL{Value: vv}, nil
	case int:
		return &types.AttributeValueMemberN{Value: strconv.Itoa(vv)}, nil
	case int8:
		return &types.AttributeValueMemberN{Value: strconv.FormatInt(int64(vv), 10)}, nil
	case int16:
		return &types.AttributeValueMemberN{Value: strconv.FormatInt(int64(vv), 10)}, nil
	case int32:
		return &types.AttributeValueMemberN{Value: strconv.FormatInt(int64(vv), 10)}, nil
	case int64:
		return &types.AttributeValueMemberN{Value: strconv.FormatInt(vv, 10)}, nil
	case uint:
		return &types.AttributeValueMemberN{Value: strconv.FormatUint(uint64(vv), 10)}, nil
	case uint8:
		return &types.AttributeValueMemberN{Value: strconv.FormatUint(uint64(vv), 10)}, nil
	case uint16:
		return &types.AttributeValueMemberN{Value: strconv.FormatUint(uint64(vv), 10)}, nil
	case uint32:
		return &types.AttributeValueMemberN{Value: strconv.FormatUint(uint64(vv), 10)}, nil
	case uint64:
		return &types.AttributeValueMemberN{Value: strconv.FormatUint(vv, 10)}, nil
	case float32:
		return &types.AttributeValueMemberN{Value: strconv.FormatFloat(float64(vv), 'f', -1, 32)}, nil
	case float64:
		return &types.AttributeValueMemberN{Value: strconv.FormatFloat(vv, 'f', -1, 64)}, nil
	case json.Number:
		// json.Number.String() preserves the source representation.
		if vv.String() == "" {
			return &types.AttributeValueMemberS{Value: ""}, nil
		}
		return &types.AttributeValueMemberN{Value: vv.String()}, nil
	case time.Time:
		// Used rarely; callers should prefer explicit pynamodb datetime formatting.
		return &types.AttributeValueMemberS{Value: vv.UTC().Format(time.RFC3339Nano)}, nil
	case []string:
		// Treat []string as a DynamoDB string set (SS). This matches ItemToInterfaceMap,
		// which converts both SS and NS into []string. We do not attempt to infer NS.
		if len(vv) == 0 {
			// DynamoDB does not allow empty sets; omit.
			return nil, nil
		}
		// Filter out empty strings (DynamoDB does not allow them in sets).
		filtered := make([]string, 0, len(vv))
		for _, s := range vv {
			if s == "" {
				continue
			}
			filtered = append(filtered, s)
		}
		if len(filtered) == 0 {
			return nil, nil
		}
		return &types.AttributeValueMemberSS{Value: filtered}, nil
	case []any:
		list := make([]types.AttributeValue, 0, len(vv))
		for _, el := range vv {
			av, err := interfaceToAV(el)
			if err != nil {
				return nil, err
			}
			if av == nil {
				// Keep list shape: represent omitted elements as NULL.
				av = &types.AttributeValueMemberNULL{Value: true}
			}
			list = append(list, av)
		}
		return &types.AttributeValueMemberL{Value: list}, nil
	case map[string]any:
		m := make(map[string]types.AttributeValue, len(vv))
		for k, v2 := range vv {
			av, err := interfaceToAV(v2)
			if err != nil {
				return nil, fmt.Errorf("map key %q: %w", k, err)
			}
			if av == nil {
				continue
			}
			m[k] = av
		}
		return &types.AttributeValueMemberM{Value: m}, nil
	default:
		// Common case when decoding JSON into map[string]any: []interface{} becomes []any already.
		// For everything else, attempt to stringify to avoid surprising crashes.
		return &types.AttributeValueMemberS{Value: fmt.Sprintf("%v", vv)}, nil
	}
}

func isNumericString(s string) bool {
	if s == "" {
		return false
	}
	if strings.TrimSpace(s) != s {
		return false
	}
	// DynamoDB does not support NaN/Inf.
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return false
	}
	if math.IsNaN(f) || math.IsInf(f, 0) {
		return false
	}
	// Reject strings that parse but contain spaces or other non-canonical forms like "+-1".
	// A conservative check: must be composed of digits and numeric punctuation.
	for i, r := range s {
		if (r >= '0' && r <= '9') || r == '-' || r == '+' || r == '.' || r == 'e' || r == 'E' {
			// '+' is only valid at start or after e/E.
			if r == '+' {
				if i != 0 {
					prev := s[i-1]
					if prev != 'e' && prev != 'E' {
						return false
					}
				}
			}
			continue
		}
		return false
	}
	return true
}
