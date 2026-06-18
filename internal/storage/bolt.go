package storage

import (
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"time"

	"go.etcd.io/bbolt"

	"apimock/internal/models"
	"apimock/pkg/utils"
)

const (
	BucketScenes       = "scenes"
	BucketRules        = "rules"
	BucketHARSessions  = "har_sessions"
	BucketSummaries    = "request_summaries"
)

type Storage struct {
	db *bbolt.DB
}

func New(dataDir string) (*Storage, error) {
	dbPath := filepath.Join(dataDir, "apimock.db")
	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{Timeout: 1 * time.Second})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		buckets := []string{BucketScenes, BucketRules, BucketHARSessions, BucketSummaries}
		for _, bucket := range buckets {
			_, err := tx.CreateBucketIfNotExists([]byte(bucket))
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create buckets: %w", err)
	}

	s := &Storage{db: db}
	if err := s.ensureDefaultScene(); err != nil {
		return nil, err
	}

	return s, nil
}

func (s *Storage) Close() error {
	return s.db.Close()
}

func (s *Storage) ensureDefaultScene() error {
	_, err := s.GetScene(models.DefaultScene)
	if errors.Is(err, ErrNotFound) {
		scene := &models.Scene{
			ID:          models.DefaultScene,
			Name:        "Default",
			Description: "Default scene",
			IsDefault:   true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		return s.SaveScene(scene)
	}
	return err
}

var ErrNotFound = errors.New("not found")

func (s *Storage) SaveScene(scene *models.Scene) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketScenes))
		data, err := json.Marshal(scene)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(scene.ID), data)
	})
}

func (s *Storage) GetScene(id string) (*models.Scene, error) {
	var scene models.Scene
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketScenes))
		data := bucket.Get([]byte(id))
		if data == nil {
			return ErrNotFound
		}
		return json.Unmarshal(data, &scene)
	})
	if err != nil {
		return nil, err
	}
	return &scene, nil
}

func (s *Storage) ListScenes() ([]*models.Scene, error) {
	var scenes []*models.Scene
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketScenes))
		return bucket.ForEach(func(k, v []byte) error {
			var scene models.Scene
			if err := json.Unmarshal(v, &scene); err != nil {
				return err
			}
			scenes = append(scenes, &scene)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return scenes, nil
}

func (s *Storage) DeleteScene(id string) error {
	if id == models.DefaultScene {
		return errors.New("cannot delete default scene")
	}
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketScenes))
		return bucket.Delete([]byte(id))
	})
}

func (s *Storage) SaveRule(rule *models.Rule) error {
	if rule.ID == "" {
		rule.ID = utils.GenerateID()
	}
	now := time.Now()
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = now
	}
	rule.UpdatedAt = now

	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketRules))
		data, err := json.Marshal(rule)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(rule.ID), data)
	})
}

func (s *Storage) GetRule(id string) (*models.Rule, error) {
	var rule models.Rule
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketRules))
		data := bucket.Get([]byte(id))
		if data == nil {
			return ErrNotFound
		}
		return json.Unmarshal(data, &rule)
	})
	if err != nil {
		return nil, err
	}
	return &rule, nil
}

func (s *Storage) ListRules(scene string) ([]*models.Rule, error) {
	var rules []*models.Rule
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketRules))
		return bucket.ForEach(func(k, v []byte) error {
			var rule models.Rule
			if err := json.Unmarshal(v, &rule); err != nil {
				return err
			}
			if scene == "" || rule.Scene == scene {
				rules = append(rules, &rule)
			}
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return rules, nil
}

func (s *Storage) DeleteRule(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketRules))
		return bucket.Delete([]byte(id))
	})
}

func (s *Storage) CheckRuleConflicts(rule *models.Rule) ([]*models.RuleConflictError, error) {
	var conflicts []*models.RuleConflictError
	rules, err := s.ListRules(rule.Scene)
	if err != nil {
		return nil, err
	}

	for _, r := range rules {
		if r.ID == rule.ID {
			continue
		}
		if r.Endpoint.Method == rule.Endpoint.Method &&
			r.Endpoint.Path == rule.Endpoint.Path &&
			r.Priority == rule.Priority {
			conflicts = append(conflicts, &models.RuleConflictError{
				Rule1ID:  rule.ID,
				Rule2ID:  r.ID,
				Endpoint: rule.Endpoint,
				Priority: rule.Priority,
			})
		}
	}
	return conflicts, nil
}

func (s *Storage) SaveHARSession(id string, har *models.HAR) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketHARSessions))
		data, err := json.Marshal(har)
		if err != nil {
			return err
		}
		return bucket.Put([]byte(id), data)
	})
}

func (s *Storage) GetHARSession(id string) (*models.HAR, error) {
	var har models.HAR
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketHARSessions))
		data := bucket.Get([]byte(id))
		if data == nil {
			return ErrNotFound
		}
		return json.Unmarshal(data, &har)
	})
	if err != nil {
		return nil, err
	}
	return &har, nil
}

func (s *Storage) ListHARSessions() ([]string, error) {
	var ids []string
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketHARSessions))
		return bucket.ForEach(func(k, v []byte) error {
			ids = append(ids, string(k))
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return ids, nil
}

func (s *Storage) DeleteHARSession(id string) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketHARSessions))
		return bucket.Delete([]byte(id))
	})
}

func (s *Storage) SaveRequestSummary(summary *models.RequestSummary) error {
	return s.db.Update(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSummaries))
		data, err := json.Marshal(summary)
		if err != nil {
			return err
		}
		key := fmt.Sprintf("%s-%d", summary.BodySHA256, summary.Timestamp.UnixNano())
		return bucket.Put([]byte(key), data)
	})
}

func (s *Storage) ListRequestSummaries() ([]*models.RequestSummary, error) {
	var summaries []*models.RequestSummary
	err := s.db.View(func(tx *bbolt.Tx) error {
		bucket := tx.Bucket([]byte(BucketSummaries))
		return bucket.ForEach(func(k, v []byte) error {
			var summary models.RequestSummary
			if err := json.Unmarshal(v, &summary); err != nil {
				return err
			}
			summaries = append(summaries, &summary)
			return nil
		})
	})
	if err != nil {
		return nil, err
	}
	return summaries, nil
}
