package cache

import (
	"fmt"
	"time"
)

// Metrics метрики кэша
type Metrics struct {
	Level1Size     int
	Level2Size     int
	Level3Size     int
	TotalSize      int
	MaxSize        int
	HitRate        float64
	EvictionCount  int
	AverageAccessTime time.Duration
}

// CacheWithMetrics расширенный интерфейс кэша с метриками
type CacheWithMetrics interface {
	Cache
	GetMetrics() Metrics
}

// FIFO3CacheWithMetrics обертка для FIFO3 кэша с метриками
type FIFO3CacheWithMetrics struct {
	Cache
	hitCount      int
	missCount     int
	evictionCount int
	totalAccessTime time.Duration
	accessCount   int
}

// NewFIFO3CacheWithMetrics создает кэш с метриками
func NewFIFO3CacheWithMetrics(maxSize int) CacheWithMetrics {
	return &FIFO3CacheWithMetrics{
		Cache: NewFIFO3Cache(maxSize),
	}
}

// Get возвращает значение с подсчетом метрик
func (c *FIFO3CacheWithMetrics) Get(key string) (interface{}, bool) {
	start := time.Now()
	value, found := c.Cache.Get(key)
	duration := time.Since(start)
	
	c.totalAccessTime += duration
	c.accessCount++
	
	if found {
		c.hitCount++
	} else {
		c.missCount++
	}
	
	return value, found
}

// Set устанавливает значение
func (c *FIFO3CacheWithMetrics) Set(key string, value interface{}) {
	// Проверяем, нужно ли вытеснение
	beforeSize := c.Cache.Size()
	c.Cache.Set(key, value)
	afterSize := c.Cache.Size()
	
	// Если размер не изменился, значит произошло вытеснение
	if beforeSize == afterSize && beforeSize >= c.Cache.MaxSize() {
		c.evictionCount++
	}
}

// GetMetrics возвращает метрики кэша
func (c *FIFO3CacheWithMetrics) GetMetrics() Metrics {
	l1, l2, l3, total := c.Cache.GetStats()
	
	hitRate := 0.0
	if c.hitCount+c.missCount > 0 {
		hitRate = float64(c.hitCount) / float64(c.hitCount+c.missCount)
	}
	
	avgAccessTime := time.Duration(0)
	if c.accessCount > 0 {
		avgAccessTime = c.totalAccessTime / time.Duration(c.accessCount)
	}
	
	return Metrics{
		Level1Size:     l1,
		Level2Size:     l2,
		Level3Size:     l3,
		TotalSize:      total,
		MaxSize:        c.Cache.MaxSize(),
		HitRate:        hitRate,
		EvictionCount:  c.evictionCount,
		AverageAccessTime: avgAccessTime,
	}
}

// String возвращает строковое представление метрик
func (m Metrics) String() string {
	return fmt.Sprintf(
		"Cache Metrics: Level1=%d, Level2=%d, Level3=%d, Total=%d/%d, HitRate=%.2f%%, Evictions=%d, AvgAccessTime=%v",
		m.Level1Size, m.Level2Size, m.Level3Size, m.TotalSize, m.MaxSize, m.HitRate*100, m.EvictionCount, m.AverageAccessTime,
	)
}