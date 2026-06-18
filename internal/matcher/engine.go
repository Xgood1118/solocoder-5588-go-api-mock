package matcher

import (
	"encoding/json"
	"math/rand"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/PaesslerAG/jsonpath"

	"apimock/internal/models"
	"apimock/pkg/utils"
)

type RequestContext struct {
	Method     string
	Path       string
	Headers    http.Header
	Query      url.Values
	Body       []byte
	ClientIP   string
	PathParams map[string]string
}

type MatcherFunc func(ctx *RequestContext, config models.MatcherConfig) bool

var matcherRegistry = map[models.MatcherType]MatcherFunc{
	models.MatcherHeader:      matchHeader,
	models.MatcherHeaderRegex: matchHeaderRegex,
	models.MatcherQuery:       matchQuery,
	models.MatcherQueryRegex:  matchQueryRegex,
	models.MatcherPath:        matchPath,
	models.MatcherBody:        matchBody,
	models.MatcherJSONPath:    matchJSONPath,
	models.MatcherRegex:       matchRegex,
	models.MatcherEquals:      matchEquals,
	models.MatcherContains:    matchContains,
	models.MatcherIPRange:     matchIPRange,
	models.MatcherTimeWindow:  matchTimeWindow,
	models.MatcherRandom:      matchRandom,
}

func Match(ctx *RequestContext, rule *models.Rule) bool {
	if len(rule.Matchers) == 0 {
		return true
	}

	logic := rule.Logic
	if logic == "" {
		logic = models.LogicAnd
	}

	return evaluateMatchers(ctx, rule.Matchers, logic)
}

func evaluateMatchers(ctx *RequestContext, matchers []models.Matcher, logic models.LogicOp) bool {
	if len(matchers) == 0 {
		return true
	}

	for _, matcher := range matchers {
		var result bool

		if len(matcher.Children) > 0 {
			childLogic := matcher.Logic
			if childLogic == "" {
				childLogic = models.LogicAnd
			}
			result = evaluateMatchers(ctx, matcher.Children, childLogic)
		} else {
			fn, ok := matcherRegistry[matcher.Type]
			if !ok {
				result = false
			} else {
				result = fn(ctx, matcher.Config)
			}
		}

		if logic == models.LogicAnd && !result {
			return false
		}
		if logic == models.LogicOr && result {
			return true
		}
	}

	return logic == models.LogicAnd
}

func matchHeader(ctx *RequestContext, config models.MatcherConfig) bool {
	value := ctx.Headers.Get(config.Key)
	if config.IgnoreCase {
		return strings.EqualFold(value, config.Value)
	}
	return value == config.Value
}

func matchHeaderRegex(ctx *RequestContext, config models.MatcherConfig) bool {
	value := ctx.Headers.Get(config.Key)
	matched, _ := regexp.MatchString(config.Pattern, value)
	return matched
}

func matchQuery(ctx *RequestContext, config models.MatcherConfig) bool {
	values := ctx.Query[config.Key]
	if len(values) == 0 {
		return false
	}
	for _, v := range values {
		if config.IgnoreCase {
			if strings.EqualFold(v, config.Value) {
				return true
			}
		} else {
			if v == config.Value {
				return true
			}
		}
	}
	return false
}

func matchQueryRegex(ctx *RequestContext, config models.MatcherConfig) bool {
	values := ctx.Query[config.Key]
	for _, v := range values {
		matched, _ := regexp.MatchString(config.Pattern, v)
		if matched {
			return true
		}
	}
	return false
}

func matchPath(ctx *RequestContext, config models.MatcherConfig) bool {
	segment := utils.GetPathSegment(ctx.Path, config.PathSegment)
	if config.IgnoreCase {
		return strings.EqualFold(segment, config.Value)
	}
	return segment == config.Value
}

func matchBody(ctx *RequestContext, config models.MatcherConfig) bool {
	bodyStr := string(ctx.Body)
	if config.IgnoreCase {
		return strings.Contains(strings.ToLower(bodyStr), strings.ToLower(config.Value))
	}
	return strings.Contains(bodyStr, config.Value)
}

func matchJSONPath(ctx *RequestContext, config models.MatcherConfig) bool {
	var data interface{}
	if err := json.Unmarshal(ctx.Body, &data); err != nil {
		return false
	}

	result, err := jsonpath.Get(config.JSONPath, data)
	if err != nil {
		return false
	}

	resultStr := toString(result)
	if config.IgnoreCase {
		return strings.EqualFold(resultStr, config.Value)
	}
	return resultStr == config.Value
}

func matchRegex(ctx *RequestContext, config models.MatcherConfig) bool {
	bodyStr := string(ctx.Body)
	matched, _ := regexp.MatchString(config.Pattern, bodyStr)
	return matched
}

func matchEquals(ctx *RequestContext, config models.MatcherConfig) bool {
	bodyStr := string(ctx.Body)
	if config.IgnoreCase {
		return strings.EqualFold(strings.TrimSpace(bodyStr), strings.TrimSpace(config.Value))
	}
	return strings.TrimSpace(bodyStr) == strings.TrimSpace(config.Value)
}

func matchContains(ctx *RequestContext, config models.MatcherConfig) bool {
	bodyStr := string(ctx.Body)
	if config.IgnoreCase {
		return strings.Contains(strings.ToLower(bodyStr), strings.ToLower(config.Value))
	}
	return strings.Contains(bodyStr, config.Value)
}

func matchIPRange(ctx *RequestContext, config models.MatcherConfig) bool {
	if config.CIDR != "" {
		return utils.IPInCIDR(ctx.ClientIP, config.CIDR)
	}
	if config.IPStart != "" && config.IPEnd != "" {
		return utils.IPInRange(ctx.ClientIP, config.IPStart, config.IPEnd)
	}
	return false
}

func matchTimeWindow(ctx *RequestContext, config models.MatcherConfig) bool {
	return utils.InTimeWindow(config.StartTime, config.EndTime)
}

func matchRandom(ctx *RequestContext, config models.MatcherConfig) bool {
	prob := config.Probability
	if prob <= 0 {
		return false
	}
	if prob >= 1 {
		return true
	}
	return rand.Float64() < prob
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	case bool:
		if val {
			return "true"
		}
		return "false"
	case nil:
		return ""
	default:
		jsonBytes, _ := json.Marshal(v)
		return string(jsonBytes)
	}
}
