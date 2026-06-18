package scene

import (
	"errors"
	"net/http"
	"sync"
	"time"

	"apimock/internal/models"
	"apimock/internal/storage"
	"apimock/pkg/utils"
)

type Manager struct {
	store        *storage.Storage
	currentScene string
	mu           sync.RWMutex
}

func NewManager(store *storage.Storage) (*Manager, error) {
	m := &Manager{
		store:        store,
		currentScene: models.DefaultScene,
	}

	if err := m.ensureBuiltinScenes(); err != nil {
		return nil, err
	}

	return m, nil
}

func (m *Manager) ensureBuiltinScenes() error {
	builtinScenes := []*models.Scene{
		{
			ID:          "normal",
			Name:        "Normal",
			Description: "Normal operation mode",
			IsDefault:   false,
		},
		{
			ID:          "empty_data",
			Name:        "Empty Data",
			Description: "Return empty datasets",
			IsDefault:   false,
		},
		{
			ID:          "error",
			Name:        "Error",
			Description: "Simulate error responses",
			IsDefault:   false,
		},
		{
			ID:          "slow",
			Name:        "Slow Response",
			Description: "Simulate slow network responses",
			IsDefault:   false,
		},
		{
			ID:          "stress",
			Name:        "Stress Test",
			Description: "High performance mode for stress testing",
			IsDefault:   false,
		},
	}

	for _, scene := range builtinScenes {
		_, err := m.store.GetScene(scene.ID)
		if errors.Is(err, storage.ErrNotFound) {
			scene.CreatedAt = time.Now()
			scene.UpdatedAt = time.Now()
			if err := m.store.SaveScene(scene); err != nil {
				return err
			}
		} else if err != nil {
			return err
		}
	}

	return nil
}

func (m *Manager) CreateScene(scene *models.Scene) error {
	if scene.ID == "" {
		scene.ID = utils.GenerateID()
	}
	scene.CreatedAt = time.Now()
	scene.UpdatedAt = time.Now()
	return m.store.SaveScene(scene)
}

func (m *Manager) GetScene(id string) (*models.Scene, error) {
	return m.store.GetScene(id)
}

func (m *Manager) ListScenes() ([]*models.Scene, error) {
	return m.store.ListScenes()
}

func (m *Manager) UpdateScene(scene *models.Scene) error {
	existing, err := m.store.GetScene(scene.ID)
	if err != nil {
		return err
	}
	scene.CreatedAt = existing.CreatedAt
	scene.UpdatedAt = time.Now()
	return m.store.SaveScene(scene)
}

func (m *Manager) DeleteScene(id string) error {
	return m.store.DeleteScene(id)
}

func (m *Manager) SetCurrentScene(sceneID string) error {
	_, err := m.store.GetScene(sceneID)
	if err != nil {
		return err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentScene = sceneID
	return nil
}

func (m *Manager) GetCurrentScene() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentScene
}

func (m *Manager) GetSceneFromRequest(r *http.Request) string {
	sceneFromHeader := r.Header.Get(models.SceneHeaderName)
	if sceneFromHeader != "" {
		_, err := m.store.GetScene(sceneFromHeader)
		if err == nil {
			return sceneFromHeader
		}
	}
	return m.GetCurrentScene()
}

func (m *Manager) SwitchScene(sceneID string) error {
	return m.SetCurrentScene(sceneID)
}
