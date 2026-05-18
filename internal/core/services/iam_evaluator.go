package services

import (
	"context"
	"net"
	"strings"
	"time"

	"github.com/poyrazk/thecloud/internal/core/domain"
)

type iamEvaluator struct{}

// NewIAMEvaluator creates a new instance of the policy evaluator.
func NewIAMEvaluator() *iamEvaluator {
	return &iamEvaluator{}
}

func (e *iamEvaluator) Evaluate(ctx context.Context, policies []*domain.Policy, action string, resource string, evalCtx map[string]interface{}) (domain.PolicyEffect, error) {
	allowFound := false

	for _, policy := range policies {
		for _, statement := range policy.Statements {
			// Evaluate conditions if present
			if len(statement.Condition) > 0 {
				if !e.evaluateCondition(statement.Condition, evalCtx) {
					continue
				}
			}

			if e.matches(statement, action, resource) {
				if statement.Effect == domain.EffectDeny {
					return domain.EffectDeny, nil
				}
				if statement.Effect == domain.EffectAllow {
					allowFound = true
				}
			}
		}
	}

	if allowFound {
		return domain.EffectAllow, nil
	}
	return "", nil // No match
}

func (e *iamEvaluator) evaluateCondition(cond domain.Condition, evalCtx map[string]interface{}) bool {
	if evalCtx == nil {
		return false
	}

	for opKey, opValues := range cond {
		op := domain.ConditionOperator(opKey)
		for key, expectedValue := range opValues {
			actualValue, exists := evalCtx[key]
			if !e.evaluateOperator(op, expectedValue, actualValue, exists) {
				return false
			}
		}
	}
	return true
}

func (e *iamEvaluator) evaluateOperator(op domain.ConditionOperator, expected, actual interface{}, exists bool) bool {
	switch op {
	case domain.CondNull:
		return e.evalNull(expected, exists)
	case domain.CondBool:
		return e.evalBool(expected, actual)
	case domain.CondStringEquals:
		return e.evalStringEquals(expected, actual, exists)
	case domain.CondStringNotEquals:
		return !e.evalStringEquals(expected, actual, exists)
	case domain.CondStringLike:
		return e.evalStringLike(expected, actual, exists)
	case domain.CondStringNotLike:
		return !e.evalStringLike(expected, actual, exists)
	case domain.CondIpAddress:
		return e.evalIP(op, expected, actual, exists)
	case domain.CondNotIpAddress:
		return !e.evalIP(op, expected, actual, exists)
	case domain.CondDateGreaterThan, domain.CondDateLessThan, domain.CondDateEquals:
		return e.evalDate(op, expected, actual, exists)
	}
	return false
}

func (e *iamEvaluator) evalNull(expected interface{}, exists bool) bool {
	// Null check: if expected is "true", key must not exist; if "false", key must exist
	if expectedStr, ok := expected.(string); ok {
		if expectedStr == "true" {
			return !exists
		}
		if expectedStr == "false" {
			return exists
		}
	}
	return false
}

func (e *iamEvaluator) evalBool(expected, actual interface{}) bool {
	expectedBool, ok := expected.(bool)
	if !ok {
		return false
	}
	actualBool, ok := actual.(bool)
	if !ok {
		return false
	}
	return expectedBool == actualBool
}

func (e *iamEvaluator) evalStringEquals(expected, actual interface{}, exists bool) bool {
	if !exists {
		return false
	}
	expectedStr, ok := expected.(string)
	if !ok {
		return false
	}
	actualStr, ok := actual.(string)
	if !ok {
		return false
	}
	return expectedStr == actualStr
}

func (e *iamEvaluator) evalStringLike(expected, actual interface{}, exists bool) bool {
	if !exists {
		return false
	}
	expectedStr, ok := expected.(string)
	if !ok {
		return false
	}
	actualStr, ok := actual.(string)
	if !ok {
		return false
	}
	return e.matchPattern(expectedStr, actualStr)
}

func (e *iamEvaluator) evalIP(_ domain.ConditionOperator, expected, actual interface{}, exists bool) bool {
	if !exists {
		return false
	}

	cidrs, ok := expected.([]interface{})
	if !ok {
		// Try single string
		cidrs = []interface{}{expected}
	}

	actualIP, ok := actual.(string)
	if !ok {
		return false
	}

	ip := net.ParseIP(actualIP)
	if ip == nil {
		return false
	}

	for _, cidrInterface := range cidrs {
		cidrStr, ok := cidrInterface.(string)
		if !ok {
			continue
		}
		_, ipNet, err := net.ParseCIDR(cidrStr)
		if err != nil {
			continue
		}
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

func (e *iamEvaluator) evalDate(op domain.ConditionOperator, expected, actual interface{}, exists bool) bool {
	if !exists {
		return false
	}

	expectedStr, ok := expected.(string)
	if !ok {
		return false
	}

	expectedTime, err := time.Parse(time.RFC3339, expectedStr)
	if err != nil {
		return false
	}

	var actualTime time.Time
	switch v := actual.(type) {
	case time.Time:
		actualTime = v
	case string:
		actualTime, err = time.Parse(time.RFC3339, v)
		if err != nil {
			return false
		}
	default:
		return false
	}

	switch op {
	case domain.CondDateGreaterThan:
		return actualTime.After(expectedTime)
	case domain.CondDateLessThan:
		return actualTime.Before(expectedTime)
	case domain.CondDateEquals:
		return actualTime.Equal(expectedTime)
	}
	return false
}

func (e *iamEvaluator) matches(statement domain.Statement, action string, resource string) bool {
	actionMatched := false
	for _, a := range statement.Action {
		if e.matchPattern(a, action) {
			actionMatched = true
			break
		}
	}

	if !actionMatched {
		return false
	}

	resourceMatched := false
	for _, r := range statement.Resource {
		if e.matchPattern(r, resource) {
			resourceMatched = true
			break
		}
	}

	return resourceMatched
}

func (e *iamEvaluator) matchPattern(pattern, target string) bool {
	if pattern == "*" {
		return true
	}

	// Simple wildcard matching: "instance:*" matches "instance:launch"
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(target, prefix)
	}

	return pattern == target
}
