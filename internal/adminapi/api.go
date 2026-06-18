package adminapi

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"

	"apimock/internal/models"
	"apimock/internal/scene"
	"apimock/internal/storage"
)

type API struct {
	store        *storage.Storage
	sceneManager *scene.Manager
}

func New(store *storage.Storage, sceneManager *scene.Manager) *API {
	return &API{
		store:        store,
		sceneManager: sceneManager,
	}
}

func (a *API) RegisterRoutes(r *gin.Engine) {
	admin := r.Group("/api/admin")
	{
		rules := admin.Group("/rules")
		{
			rules.POST("", a.CreateRule)
			rules.GET("", a.ListRules)
			rules.GET("/:id", a.GetRule)
			rules.PUT("/:id", a.UpdateRule)
			rules.DELETE("/:id", a.DeleteRule)
			rules.POST("/check-conflicts", a.CheckConflicts)
			rules.POST("/import", a.ImportRules)
			rules.GET("/export", a.ExportRules)
		}

		scenes := admin.Group("/scenes")
		{
			scenes.POST("", a.CreateScene)
			scenes.GET("", a.ListScenes)
			scenes.GET("/:id", a.GetScene)
			scenes.PUT("/:id", a.UpdateScene)
			scenes.DELETE("/:id", a.DeleteScene)
			scenes.POST("/switch", a.SwitchScene)
			scenes.GET("/current", a.GetCurrentScene)
		}

		har := admin.Group("/har")
		{
			har.GET("/sessions", a.ListHARSessions)
			har.GET("/sessions/:id", a.GetHARSession)
			har.DELETE("/sessions/:id", a.DeleteHARSession)
			har.POST("/sessions/:id/replay", a.ReplayHARSession)
			har.POST("/sessions/:id/extract-rules", a.ExtractRulesFromHAR)
		}

		summaries := admin.Group("/summaries")
		{
			summaries.GET("", a.ListRequestSummaries)
		}
	}
}

func (a *API) CreateRule(c *gin.Context) {
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conflicts, err := a.store.CheckRuleConflicts(&rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(conflicts) > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "rule conflicts detected", "conflicts": conflicts})
		return
	}

	if rule.Scene == "" {
		rule.Scene = models.DefaultScene
	}
	rule.Enabled = true

	if err := a.store.SaveRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

func (a *API) ListRules(c *gin.Context) {
	scene := c.Query("scene")
	rules, err := a.store.ListRules(scene)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (a *API) GetRule(c *gin.Context) {
	id := c.Param("id")
	rule, err := a.store.GetRule(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rule)
}

func (a *API) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	_, err := a.store.GetRule(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "rule not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	rule.ID = id

	conflicts, err := a.store.CheckRuleConflicts(&rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	if len(conflicts) > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "rule conflicts detected", "conflicts": conflicts})
		return
	}

	if err := a.store.SaveRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

func (a *API) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	if err := a.store.DeleteRule(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) CheckConflicts(c *gin.Context) {
	var rule models.Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	conflicts, err := a.store.CheckRuleConflicts(&rule)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"conflicts": conflicts})
}

func (a *API) ImportRules(c *gin.Context) {
	var rules []models.Rule
	if err := c.ShouldBindJSON(&rules); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var imported []models.Rule
	var errors []string

	for i := range rules {
		rule := &rules[i]
		conflicts, err := a.store.CheckRuleConflicts(rule)
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		if len(conflicts) > 0 {
			conflictJSON, _ := json.Marshal(conflicts)
			errors = append(errors, "conflict: "+string(conflictJSON))
			continue
		}
		if rule.Scene == "" {
			rule.Scene = models.DefaultScene
		}
		rule.Enabled = true
		if err := a.store.SaveRule(rule); err != nil {
			errors = append(errors, err.Error())
			continue
		}
		imported = append(imported, *rule)
	}

	c.JSON(http.StatusOK, gin.H{"imported": imported, "errors": errors})
}

func (a *API) ExportRules(c *gin.Context) {
	scene := c.Query("scene")
	rules, err := a.store.ListRules(scene)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, rules)
}

func (a *API) CreateScene(c *gin.Context) {
	var scene models.Scene
	if err := c.ShouldBindJSON(&scene); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.sceneManager.CreateScene(&scene); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, scene)
}

func (a *API) ListScenes(c *gin.Context) {
	scenes, err := a.sceneManager.ListScenes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, scenes)
}

func (a *API) GetScene(c *gin.Context) {
	id := c.Param("id")
	scene, err := a.sceneManager.GetScene(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "scene not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, scene)
}

func (a *API) UpdateScene(c *gin.Context) {
	id := c.Param("id")
	var scene models.Scene
	if err := c.ShouldBindJSON(&scene); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	scene.ID = id

	if err := a.sceneManager.UpdateScene(&scene); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, scene)
}

func (a *API) DeleteScene(c *gin.Context) {
	id := c.Param("id")
	if err := a.sceneManager.DeleteScene(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) SwitchScene(c *gin.Context) {
	var req struct {
		SceneID string `json:"scene_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := a.sceneManager.SwitchScene(req.SceneID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"current_scene": req.SceneID})
}

func (a *API) GetCurrentScene(c *gin.Context) {
	current := a.sceneManager.GetCurrentScene()
	scene, err := a.sceneManager.GetScene(current)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, scene)
}

func (a *API) ListHARSessions(c *gin.Context) {
	ids, err := a.store.ListHARSessions()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, ids)
}

func (a *API) GetHARSession(c *gin.Context) {
	id := c.Param("id")
	har, err := a.store.GetHARSession(id)
	if err != nil {
		if err == storage.ErrNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "HAR session not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, har)
}

func (a *API) DeleteHARSession(c *gin.Context) {
	id := c.Param("id")
	if err := a.store.DeleteHARSession(id); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

func (a *API) ReplayHARSession(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "HAR replay initiated", "id": c.Param("id")})
}

func (a *API) ExtractRulesFromHAR(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Rule extraction initiated", "id": c.Param("id")})
}

func (a *API) ListRequestSummaries(c *gin.Context) {
	summaries, err := a.store.ListRequestSummaries()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, summaries)
}
