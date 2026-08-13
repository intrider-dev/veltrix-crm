package automation

import (
	"bytes"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func PreviewRule(spec RuleSpec, event Event) (Preview, error) {
	if err := ValidateRule(spec); err != nil {
		return Preview{}, err
	}
	matched, err := Evaluate(spec.Conditions, event)
	if err != nil {
		return Preview{}, err
	}
	actions := make([]ActionType, 0, len(spec.Actions))
	if matched {
		for _, action := range spec.Actions {
			actions = append(actions, action.Type)
		}
	}
	return Preview{Matched: matched, Actions: actions}, nil
}

func Evaluate(condition Condition, event Event) (bool, error) {
	if condition.All != nil {
		for _, child := range condition.All {
			matched, err := Evaluate(child, event)
			if err != nil || !matched {
				return matched, err
			}
		}
		return true, nil
	}
	if condition.Any != nil {
		for _, child := range condition.Any {
			matched, err := Evaluate(child, event)
			if err != nil {
				return false, err
			}
			if matched {
				return true, nil
			}
		}
		return false, nil
	}
	return evaluatePredicate(condition, event)
}

func evaluatePredicate(condition Condition, event Event) (bool, error) {
	var expected any
	decoder := json.NewDecoder(bytes.NewReader(condition.Value))
	decoder.UseNumber()
	if err := decoder.Decode(&expected); err != nil {
		return false, err
	}
	switch condition.Operator {
	case ComparatorTagPresent:
		text, _ := expected.(string)
		for _, tag := range event.Tags {
			if tag == text {
				return true, nil
			}
		}
		return false, nil
	case ComparatorOwnerEquals:
		return event.OwnerID == expected, nil
	case ComparatorTeamEquals:
		return event.TeamID == expected, nil
	}
	actual, found := lookupField(event.Fields, condition.Field)
	if !found {
		return condition.Operator == ComparatorNotEquals, nil
	}
	switch condition.Operator {
	case ComparatorEquals, ComparatorNotEquals:
		equal := scalarEqual(actual, expected)
		if condition.Operator == ComparatorNotEquals {
			return !equal, nil
		}
		return equal, nil
	case ComparatorContains:
		needle, _ := expected.(string)
		return strings.Contains(strings.ToLower(fmt.Sprint(actual)), strings.ToLower(needle)), nil
	case ComparatorGreaterThan, ComparatorGreaterOrEqual, ComparatorLessThan, ComparatorLessOrEqual:
		left, leftOK := numeric(actual)
		right, rightOK := numeric(expected)
		if !leftOK || !rightOK {
			return false, nil
		}
		switch condition.Operator {
		case ComparatorGreaterThan:
			return left > right, nil
		case ComparatorGreaterOrEqual:
			return left >= right, nil
		case ComparatorLessThan:
			return left < right, nil
		default:
			return left <= right, nil
		}
	case ComparatorDateBefore, ComparatorDateAfter:
		left, leftOK := dateValue(actual)
		right, rightOK := dateValue(expected)
		if !leftOK || !rightOK {
			return false, nil
		}
		if condition.Operator == ComparatorDateBefore {
			return left.Before(right), nil
		}
		return left.After(right), nil
	default:
		return false, fmt.Errorf("unsupported comparator %q", condition.Operator)
	}
}

func lookupField(fields map[string]any, path string) (any, bool) {
	var current any = fields
	for _, part := range strings.Split(path, ".") {
		object, ok := current.(map[string]any)
		if !ok {
			return nil, false
		}
		current, ok = object[part]
		if !ok {
			return nil, false
		}
	}
	return current, true
}

func scalarEqual(left, right any) bool {
	if leftNumber, ok := numeric(left); ok {
		if rightNumber, ok := numeric(right); ok {
			return leftNumber == rightNumber
		}
	}
	return reflect.DeepEqual(left, right) || fmt.Sprint(left) == fmt.Sprint(right)
}

func numeric(value any) (float64, bool) {
	switch typed := value.(type) {
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	case int:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case string:
		number, err := strconv.ParseFloat(typed, 64)
		return number, err == nil
	default:
		return 0, false
	}
}

func dateValue(value any) (time.Time, bool) {
	switch typed := value.(type) {
	case time.Time:
		return typed.UTC(), true
	case string:
		instant, err := parseDate(typed)
		return instant, err == nil
	default:
		return time.Time{}, false
	}
}
