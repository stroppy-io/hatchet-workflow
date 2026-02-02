package k8s

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/stroppy-io/hatchet-workflow/internal/proto/crossplane"
)

// MockClient - моковый клиент для тестирования, который хранит ресурсы в памяти
// и возвращает успешный деплой каждый 3-й раз при вызове UpdateResourceFromRemote
type MockClient struct {
	mu sync.RWMutex
	// resources хранит ресурсы по ключу "namespace/name" или просто "name" для cluster-scoped ресурсов
	resources map[string]*mockResource
}

type mockResource struct {
	resource   *crossplane.Resource
	callCount  int // счётчик вызовов UpdateResourceFromRemote для этого ресурса
	externalID string
}

// NewMockClient создаёт новый моковый клиент
func NewMockClient() *MockClient {
	return &MockClient{
		resources: make(map[string]*mockResource),
	}
}

// CreateResource сохраняет ресурс в map и записывает YAML в файл
func (m *MockClient) CreateResource(
	ctx context.Context,
	resource *crossplane.Resource,
) error {
	if resource == nil {
		return fmt.Errorf("resource cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getResourceKey(resource.GetRef())

	// Генерируем fake external ID
	externalID := fmt.Sprintf("mock-id-%s", key)

	m.resources[key] = &mockResource{
		resource:   resource,
		callCount:  0,
		externalID: externalID,
	}

	// Сохраняем YAML в файл
	if err := m.saveYAMLToFile(key, resource.ResourceYaml); err != nil {
		// Логируем ошибку, но не прерываем выполнение
		fmt.Fprintf(os.Stderr, "Warning: failed to save YAML to file: %v\n", err)
	}

	return nil
}

// UpdateResourceFromRemote обновляет ресурс из "удалённого" источника (мока)
// Каждый 3-й вызов для конкретного ресурса возвращает успешный деплой
func (m *MockClient) UpdateResourceFromRemote(
	ctx context.Context,
	resource *crossplane.Resource,
) (*crossplane.Resource, error) {
	if resource == nil || resource.GetRef() == nil {
		return nil, fmt.Errorf("resource or ref cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getResourceKey(resource.GetRef())

	mockRes, exists := m.resources[key]
	if !exists {
		return nil, ErrResourceNotFound
	}

	// Увеличиваем счётчик вызовов
	mockRes.callCount++

	// Создаём копию ресурса для возврата
	updatedResource := &crossplane.Resource{
		ResourceYaml: resource.ResourceYaml,
		Ref:          resource.Ref,
		ExternalId:   mockRes.externalID,
	}

	// Каждый 3-й вызов - успешный деплой
	if mockRes.callCount%3 == 0 {
		updatedResource.Synced = true
		updatedResource.Ready = true
	} else {
		// Иначе - ещё не готов
		updatedResource.Synced = mockRes.callCount%3 >= 1 // Synced появляется раньше
		updatedResource.Ready = false
	}

	return updatedResource, nil
}

// DeleteResource удаляет ресурс из map
func (m *MockClient) DeleteResource(
	ctx context.Context,
	ref *crossplane.ExtRef,
) error {
	if ref == nil {
		return fmt.Errorf("ref cannot be nil")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.getResourceKey(ref)

	// Даже если ресурса нет, считаем это успехом (как в реальном клиенте)
	delete(m.resources, key)

	return nil
}

// getResourceKey генерирует уникальный ключ для ресурса
func (m *MockClient) getResourceKey(ref *crossplane.ExtRef) string {
	if ref == nil || ref.GetRef() == nil {
		return ""
	}

	namespace := ref.GetRef().GetNamespace()
	name := ref.GetRef().GetName()

	if namespace != "" {
		return fmt.Sprintf("%s/%s", namespace, name)
	}
	return name
}

// GetCallCount возвращает количество вызовов UpdateResourceFromRemote для ресурса (для тестирования)
func (m *MockClient) GetCallCount(ref *crossplane.ExtRef) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := m.getResourceKey(ref)
	if mockRes, exists := m.resources[key]; exists {
		return mockRes.callCount
	}
	return 0
}

// Reset очищает все сохранённые ресурсы (для тестирования)
func (m *MockClient) Reset() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.resources = make(map[string]*mockResource)
}

// saveYAMLToFile сохраняет YAML содержимое в файл в папке test_yamls
func (m *MockClient) saveYAMLToFile(key string, yamlContent string) error {
	dir := filepath.Join("tests", "yamls")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}
	fileName := filepath.Join(dir, fmt.Sprintf("%s.yaml", filepath.Base(key)))
	if key != filepath.Base(key) {
		// Если в ключе есть namespace, заменяем / на _
		safeKey := filepath.Base(filepath.Dir(key)) + "_" + filepath.Base(key)
		fileName = filepath.Join(dir, fmt.Sprintf("%s.yaml", safeKey))
	}
	if err := os.WriteFile(fileName, []byte(yamlContent), 0644); err != nil {
		return fmt.Errorf("failed to write YAML file: %w", err)
	}

	return nil
}
