package recorder

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"

	"apimock/internal/models"
	"apimock/internal/storage"
	"apimock/pkg/utils"
)

const (
	MaxFullBodySize = 1 * 1024 * 1024
	PreviewSize     = 1024
)

type Proxy struct {
	store          *storage.Storage
	targetURL      *url.URL
	activeSessions map[string]*RecordingSession
	mu             sync.RWMutex
}

type RecordingSession struct {
	ID        string
	HAR       *models.HAR
	StartTime time.Time
	TargetURL string
}

func NewProxy(store *storage.Storage) *Proxy {
	return &Proxy{
		store:          store,
		activeSessions: make(map[string]*RecordingSession),
	}
}

func (p *Proxy) SetTarget(target string) error {
	u, err := url.Parse(target)
	if err != nil {
		return err
	}
	p.targetURL = u
	return nil
}

func (p *Proxy) RegisterRoutes(r *gin.Engine) {
	api := r.Group("/api/recorder")
	{
		api.POST("/start", p.StartRecording)
		api.POST("/stop", p.StopRecording)
		api.GET("/sessions", p.ListSessions)
		api.POST("/target", p.SetTargetHandler)
	}

	r.NoRoute(p.handleProxy)
}

func (p *Proxy) SetTargetHandler(c *gin.Context) {
	var req struct {
		Target string `json:"target" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if err := p.SetTarget(req.Target); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"target": req.Target})
}

func (p *Proxy) StartRecording(c *gin.Context) {
	var req struct {
		TargetURL string `json:"target_url"`
	}
	c.ShouldBindJSON(&req)

	if req.TargetURL != "" {
		if err := p.SetTarget(req.TargetURL); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
	}

	sessionID := utils.GenerateID()
	har := &models.HAR{
		Log: models.HARLog{
			Version: "1.2",
			Creator: models.HARCreator{
				Name:    "API Mock Recorder",
				Version: "1.0",
			},
			Entries: []models.HAREntry{},
		},
	}

	session := &RecordingSession{
		ID:        sessionID,
		HAR:       har,
		StartTime: time.Now(),
		TargetURL: p.targetURL.String(),
	}

	p.mu.Lock()
	p.activeSessions[sessionID] = session
	p.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"session_id": sessionID,
		"target":     p.targetURL.String(),
	})
}

func (p *Proxy) StopRecording(c *gin.Context) {
	var req struct {
		SessionID string `json:"session_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	p.mu.Lock()
	session, exists := p.activeSessions[req.SessionID]
	if exists {
		delete(p.activeSessions, req.SessionID)
	}
	p.mu.Unlock()

	if !exists {
		c.JSON(http.StatusNotFound, gin.H{"error": "session not found"})
		return
	}

	if err := p.store.SaveHARSession(req.SessionID, session.HAR); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"session_id": req.SessionID,
		"entries":    len(session.HAR.Log.Entries),
	})
}

func (p *Proxy) ListSessions(c *gin.Context) {
	p.mu.RLock()
	sessions := make([]gin.H, 0, len(p.activeSessions))
	for id, s := range p.activeSessions {
		sessions = append(sessions, gin.H{
			"id":         id,
			"start_time": s.StartTime,
			"target":     s.TargetURL,
			"entries":    len(s.HAR.Log.Entries),
		})
	}
	p.mu.RUnlock()

	stored, _ := p.store.ListHARSessions()

	c.JSON(http.StatusOK, gin.H{
		"active": sessions,
		"stored": stored,
	})
}

func (p *Proxy) handleProxy(c *gin.Context) {
	if strings.HasPrefix(c.Request.URL.Path, "/api/recorder") {
		c.Next()
		return
	}

	if p.targetURL == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "proxy target not set"})
		return
	}

	startTime := time.Now()

	reqBody, bodyHash, bodySize, err := p.readRequestBody(c.Request)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Request.Body = io.NopCloser(bytes.NewBuffer(reqBody))

	proxy := httputil.NewSingleHostReverseProxy(p.targetURL)
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = p.targetURL.Host
	}

	var respBody bytes.Buffer
	var respStatusCode int
	var respHeader http.Header
	proxy.ModifyResponse = func(resp *http.Response) error {
		respStatusCode = resp.StatusCode
		respHeader = resp.Header.Clone()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return err
		}
		respBody.Write(body)
		resp.Body = io.NopCloser(bytes.NewBuffer(body))
		return nil
	}

	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		respStatusCode = http.StatusBadGateway
		respHeader = http.Header{}
		respBody.WriteString(err.Error())
	}

	proxy.ServeHTTP(c.Writer, c.Request)

	duration := int(time.Since(startTime).Milliseconds())

	go p.recordEntry(c.Request, reqBody, respBody.Bytes(), respStatusCode, respHeader, startTime, duration, bodyHash, bodySize)
}

func (p *Proxy) readRequestBody(req *http.Request) ([]byte, string, int64, error) {
	body, err := io.ReadAll(req.Body)
	if err != nil {
		return nil, "", 0, err
	}
	req.Body.Close()

	hash := sha256.Sum256(body)
	bodyHash := hex.EncodeToString(hash[:])
	bodySize := int64(len(body))

	return body, bodyHash, bodySize, nil
}

func (p *Proxy) recordEntry(req *http.Request, reqBody, respBody []byte, statusCode int, respHeader http.Header, startTime time.Time, duration int, bodyHash string, bodySize int64) {
	p.saveRequestSummary(req, reqBody, bodyHash, bodySize)

	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.activeSessions) == 0 {
		return
	}

	harEntry := p.createHAREntry(req, reqBody, respBody, statusCode, respHeader, startTime, duration)

	for _, session := range p.activeSessions {
		session.HAR.Log.Entries = append(session.HAR.Log.Entries, harEntry)
	}
}

func (p *Proxy) saveRequestSummary(req *http.Request, body []byte, bodyHash string, bodySize int64) {
	headerKeys := make([]string, 0, len(req.Header))
	for k := range req.Header {
		headerKeys = append(headerKeys, k)
	}

	preview := ""
	if bodySize > 0 {
		previewLen := PreviewSize
		if int(bodySize) < PreviewSize {
			previewLen = int(bodySize)
		}
		preview = string(body[:previewLen])
	}

	summary := &models.RequestSummary{
		Method:        req.Method,
		Path:          req.URL.Path,
		HeaderKeys:    headerKeys,
		BodyPreview:   preview,
		BodySHA256:    bodyHash,
		BodySize:      bodySize,
		Timestamp:     time.Now(),
		FullBodySaved: bodySize <= MaxFullBodySize,
	}

	p.store.SaveRequestSummary(summary)
}

func (p *Proxy) createHAREntry(req *http.Request, reqBody, respBody []byte, statusCode int, respHeader http.Header, startTime time.Time, duration int) models.HAREntry {
	harReq := models.HARRequest{
		Method:      req.Method,
		URL:         req.URL.String(),
		HTTPVersion: req.Proto,
		Headers:     headersToNameValue(req.Header),
		QueryString: queryToNameValue(req.URL.Query()),
		HeadersSize: int(estimateHeadersSize(req.Header)),
		BodySize:    len(reqBody),
	}

	if len(reqBody) > 0 && len(reqBody) <= MaxFullBodySize {
		harReq.PostData = &models.HARPostData{
			MimeType: req.Header.Get("Content-Type"),
			Text:     string(reqBody),
		}
	}

	harResp := models.HARResponse{
		Status:      statusCode,
		StatusText:  http.StatusText(statusCode),
		HTTPVersion: "HTTP/1.1",
		Headers:     headersToNameValue(respHeader),
		HeadersSize: int(estimateHeadersSize(respHeader)),
		BodySize:    len(respBody),
		Content: models.HARContent{
			Size:     len(respBody),
			MimeType: respHeader.Get("Content-Type"),
		},
	}

	if len(respBody) > 0 && len(respBody) <= MaxFullBodySize {
		harResp.Content.Text = string(respBody)
	}

	return models.HAREntry{
		StartedDateTime: startTime,
		Time:            duration,
		Request:         harReq,
		Response:        harResp,
		Timings: models.HARTimings{
			Blocked: -1,
			DNS:     -1,
			Connect: -1,
			Send:    0,
			Wait:    duration,
			Receive: 0,
		},
	}
}

func headersToNameValue(headers http.Header) []models.HARNameValue {
	nv := make([]models.HARNameValue, 0, len(headers))
	for k, values := range headers {
		for _, v := range values {
			nv = append(nv, models.HARNameValue{Name: k, Value: v})
		}
	}
	return nv
}

func queryToNameValue(query url.Values) []models.HARNameValue {
	nv := make([]models.HARNameValue, 0, len(query))
	for k, values := range query {
		for _, v := range values {
			nv = append(nv, models.HARNameValue{Name: k, Value: v})
		}
	}
	return nv
}

func estimateHeadersSize(headers http.Header) int64 {
	var size int64
	for k, values := range headers {
		size += int64(len(k)) + 2
		for _, v := range values {
			size += int64(len(v)) + 2
		}
	}
	return size
}
