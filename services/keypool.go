package services

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"jimeng-service/config"
	"jimeng-service/models"

	"github.com/google/uuid"
)

type KeyPool struct {
	mu       sync.RWMutex
	keys     []*models.APIKey
	dataFile string
	cfg      *config.KeyPoolConfig
}

type KeyPoolOption func(*KeyPool)

func NewKeyPool(cfg *config.KeyPoolConfig) (*KeyPool, error) {
	kp := &KeyPool{
		dataFile: cfg.DataFile,
		cfg:      cfg,
	}

	if err := kp.load(); err != nil {
		if os.IsNotExist(err) {
			if cfg.CreateSample {
				kp.createSampleKeys()
				if err := kp.save(); err != nil {
					return nil, err
				}
			}
		} else {
			return nil, err
		}
	}

	return kp, nil
}

func (kp *KeyPool) createSampleKeys() {
	defaultQuotas := kp.cfg.DefaultQuotas
	kp.keys = []*models.APIKey{
		{
			ID:        uuid.New().String(),
			AK:        "your-access-key",
			SK:        "your-secret-key",
			Name:      "示例密钥",
			Weight:    10,
			Enabled:   true,
			Functions: kp.initFunctions(true),
			Quotas: map[string]models.Quota{
				"video": {Limit: defaultQuotas.VideoSeconds, Used: 0, Enabled: true},
				"image": {Limit: defaultQuotas.ImageCount, Used: 0, Enabled: true},
			},
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		},
	}
}

func (kp *KeyPool) initFunctions(enabled bool) map[string]bool {
	funcs := make(map[string]bool)
	for _, f := range models.VideoFunctions {
		funcs[string(f)] = enabled
	}
	for _, f := range models.ImageFunctions {
		funcs[string(f)] = enabled
	}
	return funcs
}

func (kp *KeyPool) load() error {
	data, err := os.ReadFile(kp.dataFile)
	if err != nil {
		return err
	}

	var keysData models.KeysData
	if err := json.Unmarshal(data, &keysData); err != nil {
		return err
	}

	kp.keys = keysData.Keys
	return nil
}

func (kp *KeyPool) save() error {
	keysData := models.KeysData{Keys: kp.keys}
	data, err := json.MarshalIndent(keysData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(kp.dataFile, data, 0644)
}

func (kp *KeyPool) GetKeys() []*models.APIKey {
	kp.mu.RLock()
	defer kp.mu.RUnlock()
	return kp.keys
}

func (kp *KeyPool) GetKeyByID(id string) (*models.APIKey, error) {
	kp.mu.RLock()
	defer kp.mu.RUnlock()
	for _, k := range kp.keys {
		if k.ID == id {
			return k, nil
		}
	}
	return nil, errors.New("key not found")
}

func (kp *KeyPool) SelectKey(function string) (*models.APIKey, error) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	candidates := make([]*models.APIKey, 0)
	for _, k := range kp.keys {
		if k.Enabled && k.Functions[function] {
			candidates = append(candidates, k)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no available key for function: %s", function)
	}

	totalWeight := 0
	for _, k := range candidates {
		totalWeight += k.Weight
	}

	rand.Seed(time.Now().UnixNano())
	randWeight := rand.Intn(totalWeight)

	cumulative := 0
	for _, k := range candidates {
		cumulative += k.Weight
		if randWeight < cumulative {
			k.LastUsed = time.Now()
			k.UpdatedAt = time.Now()
			return k, nil
		}
	}

	return candidates[0], nil
}

func (kp *KeyPool) AddKey(ak, sk, name string, weight int) (*models.APIKey, error) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	newKey := &models.APIKey{
		ID:        uuid.New().String(),
		AK:        ak,
		SK:        sk,
		Name:      name,
		Weight:    weight,
		Enabled:   true,
		Functions: kp.initFunctions(true),
		Quotas: map[string]models.Quota{
			"video": {Limit: kp.cfg.DefaultQuotas.VideoSeconds, Used: 0, Enabled: true},
			"image": {Limit: kp.cfg.DefaultQuotas.ImageCount, Used: 0, Enabled: true},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	kp.keys = append(kp.keys, newKey)

	if err := kp.save(); err != nil {
		return nil, err
	}

	return newKey, nil
}

func (kp *KeyPool) UpdateKey(id, name string, weight int, quotas map[string]models.Quota) error {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	for _, k := range kp.keys {
		if k.ID == id {
			k.Name = name
			k.Weight = weight
			k.Quotas = quotas
			k.UpdatedAt = time.Now()
			if err := kp.save(); err != nil {
				return err
			}
			return nil
		}
	}

	return errors.New("key not found")
}

func (kp *KeyPool) DeleteKey(id string) error {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	for i, k := range kp.keys {
		if k.ID == id {
			kp.keys = append(kp.keys[:i], kp.keys[i+1:]...)
			return kp.save()
		}
	}

	return errors.New("key not found")
}

func (kp *KeyPool) MarkFunctionFailed(keyID, function string) {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	for _, k := range kp.keys {
		if k.ID == keyID {
			k.Functions[function] = false
			k.FailedCount++
			k.UpdatedAt = time.Now()

			allDisabled := true
			for _, enabled := range k.Functions {
				if enabled {
					allDisabled = false
					break
				}
			}
			if allDisabled {
				k.Enabled = false
			}

			kp.save()
			return
		}
	}
}

func (kp *KeyPool) ResetKey(id string) error {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	for _, k := range kp.keys {
		if k.ID == id {
			k.ResetFunctions()
			k.ResetQuotas()
			k.FailedCount = 0
			k.Enabled = true
			k.UpdatedAt = time.Now()
			return kp.save()
		}
	}

	return errors.New("key not found")
}

func (kp *KeyPool) ImportKeys(keys []*models.APIKey) error {
	kp.mu.Lock()
	defer kp.mu.Unlock()

	for _, k := range keys {
		if k.ID == "" {
			k.ID = uuid.New().String()
		}
		k.UpdatedAt = time.Now()
	}

	kp.keys = append(kp.keys, keys...)
	return kp.save()
}

func (kp *KeyPool) GetStatus() []map[string]interface{} {
	kp.mu.RLock()
	defer kp.mu.RUnlock()

	status := make([]map[string]interface{}, 0, len(kp.keys))
	for _, k := range kp.keys {
		keyStatus := map[string]interface{}{
			"id":           k.ID,
			"name":         k.Name,
			"enabled":      k.Enabled,
			"weight":       k.Weight,
			"failed_count": k.FailedCount,
			"last_used":    k.LastUsed,
			"quotas":       k.Quotas,
			"functions":    k.Functions,
		}
		status = append(status, keyStatus)
	}

	return status
}
