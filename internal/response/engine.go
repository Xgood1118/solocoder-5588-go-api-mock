package response

import (
	"math"
	"math/rand"
	"net/http"
	"time"

	"apimock/internal/models"
	"apimock/pkg/utils"
)

type Result struct {
	StatusCode int
	Headers    map[string]string
	Body       string
	ShouldFail bool
	FailureMsg string
	TimeoutMs  int
}

func Process(respConfig models.Response) *Result {
	result := &Result{
		StatusCode: respConfig.StatusCode,
		Headers:    respConfig.Headers,
		Body:       respConfig.Body,
	}

	if respConfig.Failure.Enabled && shouldFail(respConfig.Failure.Rate) {
		result.ShouldFail = true
		applyFailure(result, respConfig.Failure)
		return result
	}

	applyDelay(respConfig.Delay)

	return result
}

func applyDelay(delay models.DelayConfig) {
	if delay.MeanMs <= 0 {
		return
	}

	var sleepTime int

	switch delay.Type {
	case models.DelayNormal:
		mean := float64(delay.MeanMs)
		stdDev := float64(delay.StdDevMs)
		if stdDev <= 0 {
			stdDev = mean * 0.1
		}
		sleepTime = int(math.Round(utils.NormalRandom(mean, stdDev)))
		if delay.MinMs > 0 && sleepTime < delay.MinMs {
			sleepTime = delay.MinMs
		}
		if delay.MaxMs > 0 && sleepTime > delay.MaxMs {
			sleepTime = delay.MaxMs
		}
	default:
		sleepTime = delay.MeanMs
	}

	if sleepTime > 0 {
		time.Sleep(time.Duration(sleepTime) * time.Millisecond)
	}
}

func shouldFail(rate float64) bool {
	if rate <= 0 {
		return false
	}
	if rate >= 1 {
		return true
	}
	return rand.Float64() < rate
}

func applyFailure(result *Result, failure models.FailureConfig) {
	switch failure.FailureType {
	case models.Failure500:
		result.StatusCode = http.StatusInternalServerError
		result.FailureMsg = "Internal Server Error (simulated)"
	case models.Failure502:
		result.StatusCode = http.StatusBadGateway
		result.FailureMsg = "Bad Gateway (simulated)"
	case models.Failure503:
		result.StatusCode = http.StatusServiceUnavailable
		result.FailureMsg = "Service Unavailable (simulated)"
	case models.FailureTimeout:
		result.StatusCode = http.StatusGatewayTimeout
		result.FailureMsg = "Gateway Timeout (simulated)"
		if failure.TimeoutMs > 0 {
			result.TimeoutMs = failure.TimeoutMs
			time.Sleep(time.Duration(failure.TimeoutMs) * time.Millisecond)
		}
	case models.FailureRandom:
		codes := []int{
			http.StatusInternalServerError,
			http.StatusBadGateway,
			http.StatusServiceUnavailable,
			http.StatusGatewayTimeout,
		}
		result.StatusCode = codes[rand.Intn(len(codes))]
		result.FailureMsg = "Random failure (simulated)"
	default:
		result.StatusCode = http.StatusInternalServerError
		result.FailureMsg = "Internal Server Error (simulated)"
	}
}

func WriteResponse(w http.ResponseWriter, result *Result) {
	for k, v := range result.Headers {
		w.Header().Set(k, v)
	}
	w.Header().Set("X-Mock-Simulated", "true")
	if result.FailureMsg != "" {
		w.Header().Set("X-Mock-Failure", result.FailureMsg)
	}
	w.WriteHeader(result.StatusCode)
	if result.Body != "" {
		w.Write([]byte(result.Body))
	}
}
