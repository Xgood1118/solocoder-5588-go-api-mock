package mockserver

import (
	"bytes"
	"io"
	"net/http"
	"sort"
	"strings"

	"github.com/gin-gonic/gin"

	"apimock/internal/matcher"
	"apimock/internal/models"
	"apimock/internal/response"
	"apimock/internal/scene"
	"apimock/internal/storage"
	"apimock/pkg/utils"
)

type Server struct {
	store        *storage.Storage
	sceneManager *scene.Manager
}

func New(store *storage.Storage, sceneManager *scene.Manager) *Server {
	return &Server{
		store:        store,
		sceneManager: sceneManager,
	}
}

func (s *Server) RegisterRoutes(r *gin.Engine) {
	r.NoRoute(s.handleMockRequest)
}

func (s *Server) handleMockRequest(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/admin") {
		c.Next()
		return
	}

	bodyBytes, _ := io.ReadAll(c.Request.Body)
	c.Request.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	ctx := &matcher.RequestContext{
		Method:   c.Request.Method,
		Path:     c.Request.URL.Path,
		Headers:  c.Request.Header,
		Query:    c.Request.URL.Query(),
		Body:     bodyBytes,
		ClientIP: utils.GetClientIP(c.Request.RemoteAddr),
	}

	sceneName := s.sceneManager.GetSceneFromRequest(c.Request)
	rule := s.findMatchingRule(ctx, sceneName)

	if rule == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "no matching rule found",
			"scene":   sceneName,
			"method":  ctx.Method,
			"path":    ctx.Path,
		})
		return
	}

	c.Header("X-Mock-Rule-ID", rule.ID)
	c.Header("X-Mock-Rule-Name", rule.Name)
	c.Header("X-Mock-Scene", rule.Scene)

	result := response.Process(rule.Response)
	response.WriteResponse(c.Writer, result)
}

func (s *Server) findMatchingRule(ctx *matcher.RequestContext, sceneName string) *models.Rule {
	rules, err := s.store.ListRules(sceneName)
	if err != nil {
		return nil
	}

	var applicableRules []*models.Rule
	for _, rule := range rules {
		if !rule.Enabled {
			continue
		}
		if s.matchEndpoint(ctx, rule.Endpoint) {
			applicableRules = append(applicableRules, rule)
		}
	}

	sort.Slice(applicableRules, func(i, j int) bool {
		return applicableRules[i].Priority > applicableRules[j].Priority
	})

	for _, rule := range applicableRules {
		if matcher.Match(ctx, rule) {
			return rule
		}
	}

	return nil
}

func (s *Server) matchEndpoint(ctx *matcher.RequestContext, endpoint models.Endpoint) bool {
	if !strings.EqualFold(ctx.Method, endpoint.Method) {
		return false
	}

	return s.matchPath(ctx.Path, endpoint.Path)
}

func (s *Server) matchPath(requestPath, patternPath string) bool {
	requestSegs := utils.PathToSegments(requestPath)
	patternSegs := utils.PathToSegments(patternPath)

	if len(requestSegs) != len(patternSegs) {
		return false
	}

	for i := range requestSegs {
		patternSeg := patternSegs[i]
		requestSeg := requestSegs[i]

		if strings.HasPrefix(patternSeg, ":") {
			continue
		}

		if patternSeg != requestSeg {
			return false
		}
	}

	return true
}
