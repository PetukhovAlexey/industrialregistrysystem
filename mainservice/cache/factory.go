package cache

// CacheType тип кэша
type CacheType string

const (
	FIFO3CacheType CacheType = "fifo3"
	// Можно добавить другие типы кэшей в будущем
	// LRUCacheType    CacheType = "lru"
	// LFUCacheType    CacheType = "lfu"
)

// Config конфигурация для создания кэша
type Config struct {
	Type    CacheType
	MaxSize int
}

// NewCache создает новый кэш по конфигурации
func NewCache(config Config) Cache {
	switch config.Type {
	case FIFO3CacheType:
		return NewFIFO3Cache(config.MaxSize)
	default:
		// По умолчанию используем FIFO3
		return NewFIFO3Cache(config.MaxSize)
	}
}

// NewDefaultCache создает кэш с настройками по умолчанию
func NewDefaultCache() Cache {
	return NewFIFO3Cache(1000)
}