package kb

import (
	"container/list"
	"sync"
)

// 代理上下文緩存：LRU（依條目數與總位元組雙上限）緩存已組裝的 ContextBundle。
// 鍵 = 查詢指紋 + 資料版本 + 檢索參數——同查詢、同資料、同參數必命中；
// 資料版本（versionOf）變化時舊鍵自然失效（版本含於鍵中）。
type cache struct {
	mu         sync.Mutex
	maxEntries int
	maxBytes   int64
	entries    map[string]*list.Element
	lru        *list.List // front = 最近使用
	bytes      int64
	hits       uint64
	misses     uint64
}

type cacheEntry struct {
	key   string
	val   *ContextBundle
	bytes int64
}

func newCache(maxEntries, maxBytes int) *cache {
	if maxEntries <= 0 {
		maxEntries = 256
	}
	if maxBytes <= 0 {
		maxBytes = 8 << 20 // 8 MiB
	}
	return &cache{
		maxEntries: maxEntries,
		maxBytes:   int64(maxBytes),
		entries:    map[string]*list.Element{},
		lru:        list.New(),
	}
}

func (c *cache) get(key string) (*ContextBundle, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		c.lru.MoveToFront(el)
		c.hits++
		return el.Value.(*cacheEntry).val, true
	}
	c.misses++
	return nil, false
}

func (c *cache) put(key string, val *ContextBundle, bytes int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.entries[key]; ok {
		c.lru.MoveToFront(el)
		return
	}
	el := c.lru.PushFront(&cacheEntry{key: key, val: val, bytes: bytes})
	c.entries[key] = el
	c.bytes += bytes
	for (c.lru.Len() > c.maxEntries || c.bytes > c.maxBytes) && c.lru.Len() > 0 {
		back := c.lru.Back()
		if back == nil {
			break
		}
		c.lru.Remove(back)
		ce := back.Value.(*cacheEntry)
		delete(c.entries, ce.key)
		c.bytes -= ce.bytes
	}
}

type cacheStats struct {
	Entries    int                  `json:"entries"`
	Bytes      int64                `json:"bytes"`
	Hits       uint64               `json:"hits"`
	Misses     uint64               `json:"misses"`
	Persistent persistentCacheStats `json:"persistent,omitempty"`
}

func (c *cache) stats() cacheStats {
	c.mu.Lock()
	defer c.mu.Unlock()
	return cacheStats{Entries: c.lru.Len(), Bytes: c.bytes, Hits: c.hits, Misses: c.misses}
}
