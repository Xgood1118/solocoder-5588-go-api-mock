package har

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"

	"apimock/internal/models"
	"apimock/internal/storage"
	"apimock/pkg/utils"
)

type Processor struct {
	store *storage.Storage
}

func NewProcessor(store *storage.Storage) *Processor {
	return &Processor{store: store}
}

func (p *Processor) ParseHAR(data []byte) (*models.HAR, error) {
	var har models.HAR
	if err := json.Unmarshal(data, &har); err != nil {
		return nil, fmt.Errorf("failed to parse HAR: %w", err)
	}
	return &har, nil
}

func (p *Processor) ExtractRules(har *models.HAR, scene string, basePriority int) ([]*models.Rule, []error) {
	var rules []*models.Rule
	var errs []error

	endpointMap := make(map[string]int)

	for i, entry := range har.Log.Entries {
		rule, err := p.entryToRule(entry, scene, basePriority+i)
		if err != nil {
			errs = append(errs, err)
			continue
		}

		endpointKey := fmt.Sprintf("%s:%s", rule.Endpoint.Method, rule.Endpoint.Path)
		if idx, exists := endpointMap[endpointKey]; exists {
			existingRule := rules[idx]
			existingRule.Priority = rule.Priority
			if entry.Time > 0 {
				existingRule.Response.Delay.MeanMs = entry.Time
			}
			continue
		}

		endpointMap[endpointKey] = len(rules)
		rules = append(rules, rule)
	}

	return rules, errs
}

func (p *Processor) entryToRule(entry models.HAREntry, scene string, priority int) (*models.Rule, error) {
	parsedURL, err := url.Parse(entry.Request.URL)
	if err != nil {
		return nil, err
	}

	path := parsedURL.Path
	if path == "" {
		path = "/"
	}

	matchers := p.createMatchers(entry.Request)

	headers := make(map[string]string)
	for _, h := range entry.Response.Headers {
		if !strings.HasPrefix(h.Name, ":") && !isHopHeader(h.Name) {
			headers[h.Name] = h.Value
		}
	}

	body := ""
	if entry.Response.Content.Text != "" {
		body = entry.Response.Content.Text
	}

	delayMs := entry.Time
	if delayMs < 0 {
		delayMs = 0
	}

	rule := &models.Rule{
		ID:       utils.GenerateID(),
		Name:     fmt.Sprintf("%s %s", entry.Request.Method, path),
		Scene:    scene,
		Endpoint: models.Endpoint{
			Method: entry.Request.Method,
			Path:   path,
		},
		Priority: priority,
		Matchers: matchers,
		Logic:    models.LogicAnd,
		Response: models.Response{
			StatusCode: entry.Response.Status,
			Headers:    headers,
			Body:       body,
			Delay: models.DelayConfig{
				Type:   models.DelayFixed,
				MeanMs: delayMs,
			},
		},
		Enabled:   true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	return rule, nil
}

func (p *Processor) createMatchers(req models.HARRequest) []models.Matcher {
	var matchers []models.Matcher

	if len(req.QueryString) > 0 {
		for _, q := range req.QueryString {
			matchers = append(matchers, models.Matcher{
				Type: models.MatcherQuery,
				Config: models.MatcherConfig{
					Key:   q.Name,
					Value: q.Value,
				},
			})
		}
	}

	if req.PostData != nil && req.PostData.Text != "" {
		matchers = append(matchers, models.Matcher{
			Type: models.MatcherBody,
			Config: models.MatcherConfig{
				Value: req.PostData.Text[:minInt(100, len(req.PostData.Text))],
			},
		})
	}

	return matchers
}

func (p *Processor) ImportAndSaveRules(har *models.HAR, scene string) (*ImportResult, error) {
	if scene == "" {
		scene = models.DefaultScene
	}

	rules, errs := p.ExtractRules(har, scene, 100)

	var imported []*models.Rule
	var skipped []string

	for _, rule := range rules {
		conflicts, err := p.store.CheckRuleConflicts(rule)
		if err != nil {
			errs = append(errs, err)
			skipped = append(skipped, fmt.Sprintf("%s %s: conflict check error", rule.Endpoint.Method, rule.Endpoint.Path))
			continue
		}
		if len(conflicts) > 0 {
			skipped = append(skipped, fmt.Sprintf("%s %s: priority conflict", rule.Endpoint.Method, rule.Endpoint.Path))
			continue
		}

		if err := p.store.SaveRule(rule); err != nil {
			errs = append(errs, err)
			skipped = append(skipped, fmt.Sprintf("%s %s: %s", rule.Endpoint.Method, rule.Endpoint.Path, err.Error()))
			continue
		}

		imported = append(imported, rule)
	}

	return &ImportResult{
		Imported: imported,
		Skipped:  skipped,
		Errors:   errs,
	}, nil
}

func (p *Processor) ReplaySession(sessionID string) error {
	har, err := p.store.GetHARSession(sessionID)
	if err != nil {
		return err
	}

	return p.Replay(har)
}

func (p *Processor) Replay(har *models.HAR) error {
	if len(har.Log.Entries) == 0 {
		return errors.New("no entries to replay")
	}

	rules, errs := p.ExtractRules(har, models.DefaultScene, 200)
	if len(errs) > 0 {
		return fmt.Errorf("extraction errors: %v", errs)
	}

	for _, rule := range rules {
		conflicts, err := p.store.CheckRuleConflicts(rule)
		if err != nil {
			return err
		}
		if len(conflicts) > 0 {
			continue
		}
		p.store.SaveRule(rule)
	}

	return nil
}

func (p *Processor) SaveSession(id string, har *models.HAR) error {
	return p.store.SaveHARSession(id, har)
}

func (p *Processor) GetSession(id string) (*models.HAR, error) {
	return p.store.GetHARSession(id)
}

type ImportResult struct {
	Imported []*models.Rule `json:"imported"`
	Skipped  []string       `json:"skipped"`
	Errors   []error        `json:"errors,omitempty"`
}

func isHopHeader(name string) bool {
	hopHeaders := map[string]bool{
		"Connection":          true,
		"Keep-Alive":          true,
		"Proxy-Authenticate":  true,
		"Proxy-Authorization": true,
		"TE":                true,
		"Trailers":          true,
		"Transfer-Encoding": true,
		"Upgrade":           true,
	}
	return hopHeaders[name]
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}
