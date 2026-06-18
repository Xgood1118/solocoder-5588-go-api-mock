package models

type DelayType string

const (
	DelayFixed     DelayType = "fixed"
	DelayNormal    DelayType = "normal"
)

type FailureType string

const (
	Failure500    FailureType = "500"
	Failure502    FailureType = "502"
	Failure503    FailureType = "503"
	FailureTimeout FailureType = "timeout"
	FailureRandom FailureType = "random"
)

type Response struct {
	StatusCode int                 `json:"status_code"`
	Headers    map[string]string   `json:"headers,omitempty"`
	Body       string              `json:"body,omitempty"`
	BodyFile   string              `json:"body_file,omitempty"`
	Delay      DelayConfig         `json:"delay,omitempty"`
	Failure    FailureConfig       `json:"failure,omitempty"`
}

type DelayConfig struct {
	Type     DelayType `json:"type"`
	MeanMs   int       `json:"mean_ms"`
	StdDevMs int       `json:"std_dev_ms,omitempty"`
	MinMs    int       `json:"min_ms,omitempty"`
	MaxMs    int       `json:"max_ms,omitempty"`
}

type FailureConfig struct {
	Enabled       bool        `json:"enabled"`
	Rate          float64     `json:"rate"`
	FailureType   FailureType `json:"failure_type"`
	TimeoutMs     int         `json:"timeout_ms,omitempty"`
}
