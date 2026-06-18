package models

type MatcherType string

const (
	MatcherHeader       MatcherType = "header"
	MatcherHeaderRegex  MatcherType = "header_regex"
	MatcherQuery        MatcherType = "query"
	MatcherQueryRegex   MatcherType = "query_regex"
	MatcherPath         MatcherType = "path"
	MatcherBody         MatcherType = "body"
	MatcherJSONPath     MatcherType = "jsonpath"
	MatcherRegex        MatcherType = "regex"
	MatcherEquals       MatcherType = "equals"
	MatcherContains     MatcherType = "contains"
	MatcherIPRange      MatcherType = "ip_range"
	MatcherTimeWindow   MatcherType = "time_window"
	MatcherRandom       MatcherType = "random"
)

type LogicOp string

const (
	LogicAnd LogicOp = "AND"
	LogicOr  LogicOp = "OR"
)

type Matcher struct {
	Type     MatcherType       `json:"type"`
	Config   MatcherConfig     `json:"config"`
	Children []Matcher         `json:"children,omitempty"`
	Logic    LogicOp           `json:"logic,omitempty"`
}

type MatcherConfig struct {
	Key          string   `json:"key,omitempty"`
	Value        string   `json:"value,omitempty"`
	Values       []string `json:"values,omitempty"`
	Pattern      string   `json:"pattern,omitempty"`
	JSONPath     string   `json:"jsonpath,omitempty"`
	IPStart      string   `json:"ip_start,omitempty"`
	IPEnd        string   `json:"ip_end,omitempty"`
	CIDR         string   `json:"cidr,omitempty"`
	StartTime    string   `json:"start_time,omitempty"`
	EndTime      string   `json:"end_time,omitempty"`
	Probability  float64  `json:"probability,omitempty"`
	IgnoreCase   bool     `json:"ignore_case,omitempty"`
	PathSegment  int      `json:"path_segment,omitempty"`
}
