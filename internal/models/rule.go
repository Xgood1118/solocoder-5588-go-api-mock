package models

import "time"

type Endpoint struct {
	Method string `json:"method"`
	Path   string `json:"path"`
}

type Rule struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Scene      string     `json:"scene"`
	Endpoint   Endpoint   `json:"endpoint"`
	Priority   int        `json:"priority"`
	Matchers   []Matcher  `json:"matchers"`
	Logic      LogicOp    `json:"logic,omitempty"`
	Response   Response   `json:"response"`
	Enabled    bool       `json:"enabled"`
	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

type RuleConflictError struct {
	Rule1ID   string `json:"rule1_id"`
	Rule2ID   string `json:"rule2_id"`
	Endpoint  Endpoint `json:"endpoint"`
	Priority  int    `json:"priority"`
}
