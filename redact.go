package liberlogger

import (
	"bytes"
	"encoding/json"
	"math"
	"reflect"
	"strings"
)

func Redact(keysToRedact []string, keysToMask []string, body interface{}) interface{} {
	if body == nil {
		return nil
	}
	switch bodyParse := body.(type) {
	case bytes.Buffer:
		var parse interface{}
		err := json.Unmarshal(bodyParse.Bytes(), &parse)
		if err != nil {
			return nil
		}
		return Redact(keysToRedact, keysToMask, parse)
	case string:
		return map[string]interface{}{
			"plain/text-type": bodyParse,
		}
	case []interface{}:
		newBody := []interface{}{}

		for _, value := range bodyParse {
			newBody = append(newBody, Redact(keysToRedact, keysToMask, value))
		}

		return newBody
	default:
		return redact(keysToRedact, keysToMask, body, "")
	}
}

func overlay(str string, overlay string, start int, end int) (overlayed string) {
	r := []rune(str)
	l := len([]rune(r))

	if l == 0 {
		return ""
	}

	if start < 0 {
		start = 0
	}
	if start > l {
		start = l
	}
	if end < 0 {
		end = 0
	}
	if end > l {
		end = l
	}
	if start > end {
		tmp := start
		start = end
		end = tmp
	}

	overlayed = ""
	overlayed += string(r[:start])
	overlayed += overlay
	overlayed += string(r[end:])
	return overlayed
}

func strLoop(str string, length int) string {
	var mask string
	for i := 1; i <= length; i++ {
		mask += str
	}
	return mask
}

func maskValue(value string) string {
	valueLength := len([]rune(value))
	if valueLength == 0 {
		return ""
	}
	if valueLength == 1 {
		return value
	}
	lengthToMask := int(math.Ceil(float64(valueLength) / 3))
	return overlay(value, strLoop("*", lengthToMask), lengthToMask, lengthToMask*2)
}

func redactValue(keysToRedact []string, keysToMask []string, field string, value interface{}) interface{} {
	if value == nil {
		return value
	}
	for _, keyRedact := range keysToRedact {
		if strings.ToLower(field) == strings.ToLower(keyRedact) && !ignoreRedacted() {
			return REDACTED
		}
	}
	for _, keyMask := range keysToMask {
		if strings.ToLower(field) == strings.ToLower(keyMask) {
			return maskValue(value.(string))
		}
	}
	return value
}

func redact(keysToRedact []string, keysToMask []string, input interface{}, field string) interface{} {
	inputValue := reflect.ValueOf(input)
	inputType := reflect.TypeOf(input)
	newBody := map[string]interface{}{}

	switch inputType.Kind() {
	case reflect.Map:
		for _, index := range inputValue.MapKeys() {
			value := inputValue.MapIndex(index)
			fieldName := index.String()
			if !value.IsValid() {
				continue
			}

			if value.Interface() == nil {
				continue
			}

			newBody[fieldName] = redact(keysToRedact, keysToMask, value.Interface(), fieldName)
		}
	case reflect.Struct:
		for i := 0; i < inputType.NumField(); i++ {
			value := inputValue.Field(i)
			fieldName := inputType.Field(i).Name
			if !value.IsValid() {
				continue
			}

			if value.Interface() == nil {
				continue
			}

			newBody[fieldName] = redact(keysToRedact, keysToMask, value.Interface(), fieldName)
		}
	case reflect.Ptr:
		if !inputValue.IsNil() {
			return redact(keysToRedact, keysToMask, inputValue.Elem().Interface(), field)
		}
	default:
		if field == "" {
			return input
		}
		return redactValue(keysToRedact, keysToMask, field, input)
	}

	return newBody
}
