package mermaid

import (
	"container/list"
	"image"
	"sync"
)

// Cache is a thread-safe LRU cache of rendered diagram images and of
// render errors, keyed by mermaid.Key over src, theme and scale.
//
// Errors are cached too: a broken diagram is deterministic, so retrying
// it every frame buys nothing; a document reload changes the source and
// with it the key. A syntax error thus renders once per key, not once
// per frame.
//
// Concurrency: one mutex guards the list and the map; rendering runs
// outside the lock, so two goroutines may render the same key at once and
// one result wins. Acceptable for a viewer; documented, not perfect.
type Cache struct {
	mu       sync.Mutex
	capacity int
	order    *list.List // front = most recently used, back = least recently used
	items    map[string]*list.Element
}

type cacheEntry struct {
	key string
	img image.Image
	err error
}

// NewCache returns an LRU cache holding up to capacity images. A capacity
// of 0 or less disables caching.
func NewCache(capacity int) *Cache {
	return &Cache{
		capacity: capacity,
		order:    list.New(),
		items:    make(map[string]*list.Element),
	}
}

// Render returns the cached image for these inputs, or renders, caches
// and returns it. Render errors are cached by the same key: the second
// call returns the stored error without rendering again.
func (c *Cache) Render(src string, theme Theme, scale float64) (image.Image, error) {
	key := Key(src, theme, scale)

	c.mu.Lock()
	if el, ok := c.items[key]; ok {
		c.order.MoveToFront(el)
		c.mu.Unlock()
		e := el.Value.(*cacheEntry)
		return e.img, e.err
	}
	c.mu.Unlock()

	img, err := Render(src, theme, scale)

	c.mu.Lock()
	defer c.mu.Unlock()
	if c.capacity <= 0 {
		return img, err
	}
	if el, ok := c.items[key]; ok {
		e := el.Value.(*cacheEntry)
		e.img, e.err = img, err
		c.order.MoveToFront(el)
	} else {
		c.items[key] = c.order.PushFront(&cacheEntry{key: key, img: img, err: err})
	}
	for len(c.items) > c.capacity {
		c.evictLocked()
	}
	return img, err
}

// evictLocked removes the least recently used entry; caller holds mu.
func (c *Cache) evictLocked() {
	el := c.order.Back()
	if el == nil {
		return
	}
	c.order.Remove(el)
	delete(c.items, el.Value.(*cacheEntry).key)
}
