package cache

import (
	"sync"
	"time"
)

// Cache интерфейс для универсального кэша
type Cache interface {
	// Get возвращает значение по ключу и флаг наличия
	Get(key string) (interface{}, bool)
	
	// Set устанавливает значение по ключу
	Set(key string, value interface{})
	
	// Remove удаляет значение по ключу
	Remove(key string)
	
	// Clear очищает весь кэш
	Clear()
	
	// GetStats возвращает статистику кэша
	GetStats() (int, int, int, int)
	
	// Size возвращает текущий размер кэша
	Size() int
	
	// MaxSize возвращает максимальный размер кэша
	MaxSize() int
}

// CacheItem представляет элемент кэша
type CacheItem struct {
	key         string
	value       interface{}
	createdAt   time.Time
	lastAccess  time.Time
	accessCount int
}

// FIFO3Cache реализует алгоритм FIFO с тремя уровнями приоритета
type FIFO3Cache struct {
	mu          sync.RWMutex
	level1      map[string]*CacheItem // Горячие данные (часто запрашиваемые)
	level2      map[string]*CacheItem // Теплые данные
	level3      map[string]*CacheItem // Холодные данные
	maxSize     int
	currentSize int
}

// NewFIFO3Cache создает новый FIFO3 кэш
func NewFIFO3Cache(maxSize int) Cache {
	return &FIFO3Cache{
		level1:  make(map[string]*CacheItem),
		level2:  make(map[string]*CacheItem),
		level3:  make(map[string]*CacheItem),
		maxSize: maxSize,
	}
}

// Get возвращает значение по ключу
func (cache *FIFO3Cache) Get(key string) (interface{}, bool) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	if item, found := cache.level1[key]; found {
		item.lastAccess = time.Now()
		item.accessCount++
		return item.value, true
	}

	if item, found := cache.level2[key]; found {
		item.lastAccess = time.Now()
		item.accessCount++
		if item.accessCount > 5 {
			cache.promoteToLevel1(key, item)
		}
		return item.value, true
	}

	if item, found := cache.level3[key]; found {
		item.lastAccess = time.Now()
		item.accessCount++
		if item.accessCount > 3 {
			cache.promoteToLevel2(key, item)
		}
		return item.value, true
	}

	return nil, false
}

// Set устанавливает значение по ключу
func (cache *FIFO3Cache) Set(key string, value interface{}) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	// Обновляем существующий элемент если есть
	if _, exists := cache.level1[key]; exists {
		cache.level1[key].value = value
		cache.level1[key].lastAccess = time.Now()
		return
	}
	if _, exists := cache.level2[key]; exists {
		cache.level2[key].value = value
		cache.level2[key].lastAccess = time.Now()
		return
	}
	if _, exists := cache.level3[key]; exists {
		cache.level3[key].value = value
		cache.level3[key].lastAccess = time.Now()
		return
	}

	// Создаем новый элемент
	item := &CacheItem{
		key:         key,
		value:       value,
		createdAt:   time.Now(),
		lastAccess:  time.Now(),
		accessCount: 1,
	}

	// Вытесняем элемент если нужно
	if cache.currentSize >= cache.maxSize {
		cache.evict()
	}

	cache.level3[key] = item
	cache.currentSize++
}

// Remove удаляет значение по ключу
func (cache *FIFO3Cache) Remove(key string) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	if _, exists := cache.level1[key]; exists {
		delete(cache.level1, key)
		cache.currentSize--
		return
	}
	if _, exists := cache.level2[key]; exists {
		delete(cache.level2, key)
		cache.currentSize--
		return
	}
	if _, exists := cache.level3[key]; exists {
		delete(cache.level3, key)
		cache.currentSize--
		return
	}
}

// Clear очищает весь кэш
func (cache *FIFO3Cache) Clear() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.level1 = make(map[string]*CacheItem)
	cache.level2 = make(map[string]*CacheItem)
	cache.level3 = make(map[string]*CacheItem)
	cache.currentSize = 0
}

// GetStats возвращает статистику кэша
func (cache *FIFO3Cache) GetStats() (int, int, int, int) {
	cache.mu.RLock()
	defer cache.mu.RUnlock()

	return len(cache.level1), len(cache.level2), len(cache.level3), cache.currentSize
}

// Size возвращает текущий размер кэша
func (cache *FIFO3Cache) Size() int {
	cache.mu.RLock()
	defer cache.mu.RUnlock()
	return cache.currentSize
}

// MaxSize возвращает максимальный размер кэша
func (cache *FIFO3Cache) MaxSize() int {
	return cache.maxSize
}

// promoteToLevel1 перемещает элемент на уровень 1
func (cache *FIFO3Cache) promoteToLevel1(key string, item *CacheItem) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	delete(cache.level2, key)
	cache.level1[key] = item
}

// promoteToLevel2 перемещает элемент на уровень 2
func (cache *FIFO3Cache) promoteToLevel2(key string, item *CacheItem) {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	delete(cache.level3, key)
	cache.level2[key] = item
}

// evict вытесняет один элемент по алгоритму FIFO
func (cache *FIFO3Cache) evict() {
	if len(cache.level3) > 0 {
		for key := range cache.level3 {
			delete(cache.level3, key)
			cache.currentSize--
			return
		}
	}

	if len(cache.level2) > 0 {
		for key := range cache.level2 {
			delete(cache.level2, key)
			cache.currentSize--
			return
		}
	}

	if len(cache.level1) > 0 {
		for key := range cache.level1 {
			delete(cache.level1, key)
			cache.currentSize--
			return
		}
	}
}