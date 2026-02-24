package rendercache

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

type Renderer func(ctx context.Context) ([]byte, error)

type Options struct {
	// MaxBytes is total memory budget for cached HTML (required).
	MaxBytes int

	// MaxEntryBytes limits a single rendered HTML size (0 = unlimited).
	MaxEntryBytes int

	// TTL is optional time-to-live for cache entries (0 = no TTL).
	TTL time.Duration

	// OnEvict is optional callback when an entry is evicted/purged.
	OnEvict func(key string, reason EvictReason, bytes int)
}

type EvictReason string

const (
	EvictReasonLRU      EvictReason = "lru"
	EvictReasonTTL      EvictReason = "ttl"
	EvictReasonPurge    EvictReason = "purge"
	EvictReasonOversize EvictReason = "oversize"
)

type Cache struct {
	ver string
	opt Options

	sf singleflight.Group

	mu    sync.Mutex
	ll    *list.List // front=most recent
	items map[string]*list.Element
	used  int

	// docID -> key set, used for targeted purge when a doc updates
	docIndex map[string]map[string]struct{}
}

type entry struct {
	key       string
	docID     string
	html      []byte
	bytes     int
	createdAt time.Time
	lastHitAt time.Time
}

// New creates a render cache. rendererVersion should change when renderer/config changes.
func New(rendererVersion string, opt Options) (*Cache, error) {
	if opt.MaxBytes <= 0 {
		return nil, errors.New("MaxBytes must be > 0")
	}
	c := &Cache{
		ver:      rendererVersion,
		opt:      opt,
		ll:       list.New(),
		items:    make(map[string]*list.Element),
		docIndex: make(map[string]map[string]struct{}),
	}
	return c, nil
}

// BuildKey builds a stable, versioned cache key using docID + updatedAt + rendererVersion
// plus optional "vary" parts that affect rendered HTML (e.g. theme, feature flags).
func (c *Cache) BuildKey(docID string, updatedAt time.Time, vary ...string) string {
	raw := "doc=" + docID +
		"|u=" + strconv.FormatInt(updatedAt.UnixNano(), 10) +
		"|rv=" + c.ver
	for _, v := range vary {
		if v == "" {
			continue
		}
		raw += "|v=" + v
	}
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// GetOrRender returns cached HTML if present, otherwise renders once (singleflight), caches it and returns it.
// IMPORTANT: docID is required so we can maintain docIndex for PurgeDoc.
func (c *Cache) GetOrRender(ctx context.Context, docID, key string, render Renderer) ([]byte, error) {
	if html, ok := c.get(key); ok {
		return html, nil
	}

	v, err, _ := c.sf.Do(key, func() (any, error) {
		// double-check after entering singleflight
		if html, ok := c.get(key); ok {
			return html, nil
		}

		html, err := render(ctx)
		if err != nil {
			return nil, err
		}
		if html == nil {
			return nil, errors.New("renderer returned nil html")
		}
		if c.opt.MaxEntryBytes > 0 && len(html) > c.opt.MaxEntryBytes {
			c.evictCallback(key, EvictReasonOversize, len(html))
			// 超大页面不入缓存，但不影响本次请求返回结果。
			return html, nil
		}

		c.add(docID, key, html)
		return html, nil
	})
	if err != nil {
		return nil, err
	}
	return v.([]byte), nil
}

// Purge removes one key if present.
func (c *Cache) Purge(key string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if ele, ok := c.items[key]; ok {
		c.removeElementLocked(ele, EvictReasonPurge)
	}
}

// PurgeDoc removes ALL cached entries for a document (all versions / vary keys) immediately.
// Use this after a doc update if you want proactive cleanup.
func (c *Cache) PurgeDoc(docID string) int {
	c.mu.Lock()
	defer c.mu.Unlock()

	set, ok := c.docIndex[docID]
	if !ok || len(set) == 0 {
		return 0
	}

	// Copy keys first to avoid map mutation issues while removing.
	keys := make([]string, 0, len(set))
	for k := range set {
		keys = append(keys, k)
	}

	n := 0
	for _, k := range keys {
		if ele, ok := c.items[k]; ok {
			c.removeElementLocked(ele, EvictReasonPurge)
			n++
		} else {
			// stale index entry; just drop it
			delete(set, k)
		}
	}

	if len(set) == 0 {
		delete(c.docIndex, docID)
	}
	return n
}

// Stats returns current memory usage and entry count.
func (c *Cache) Stats() (entries int, usedBytes int, maxBytes int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.items), c.used, c.opt.MaxBytes
}

func (c *Cache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	ele, ok := c.items[key]
	if !ok {
		return nil, false
	}
	ent := ele.Value.(*entry)

	// TTL check
	if c.opt.TTL > 0 && time.Since(ent.createdAt) > c.opt.TTL {
		c.removeElementLocked(ele, EvictReasonTTL)
		return nil, false
	}

	ent.lastHitAt = time.Now()
	c.ll.MoveToFront(ele)
	return ent.html, true
}

func (c *Cache) add(docID, key string, html []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	if ele, ok := c.items[key]; ok {
		ent := ele.Value.(*entry)

		// If docID mismatches, fix index (should not happen if caller is correct)
		if ent.docID != docID {
			c.indexRemoveLocked(ent.docID, key)
			ent.docID = docID
			c.indexAddLocked(docID, key)
		}

		// Update bytes
		c.used -= ent.bytes
		ent.html = html
		ent.bytes = len(html)
		ent.createdAt = now
		ent.lastHitAt = now
		c.used += ent.bytes

		c.ll.MoveToFront(ele)
	} else {
		ent := &entry{
			key:       key,
			docID:     docID,
			html:      html,
			bytes:     len(html),
			createdAt: now,
			lastHitAt: now,
		}
		ele := c.ll.PushFront(ent)
		c.items[key] = ele
		c.used += ent.bytes
		c.indexAddLocked(docID, key)
	}

	// Evict until within budget
	for c.used > c.opt.MaxBytes {
		oldest := c.ll.Back()
		if oldest == nil {
			break
		}
		c.removeElementLocked(oldest, EvictReasonLRU)
	}
}

func (c *Cache) removeElementLocked(ele *list.Element, reason EvictReason) {
	ent := ele.Value.(*entry)

	delete(c.items, ent.key)
	c.ll.Remove(ele)
	c.used -= ent.bytes

	c.indexRemoveLocked(ent.docID, ent.key)
	c.evictCallback(ent.key, reason, ent.bytes)
}

func (c *Cache) indexAddLocked(docID, key string) {
	set, ok := c.docIndex[docID]
	if !ok {
		set = make(map[string]struct{}, 4)
		c.docIndex[docID] = set
	}
	set[key] = struct{}{}
}

func (c *Cache) indexRemoveLocked(docID, key string) {
	set, ok := c.docIndex[docID]
	if !ok {
		return
	}
	delete(set, key)
	if len(set) == 0 {
		delete(c.docIndex, docID)
	}
}

func (c *Cache) evictCallback(key string, reason EvictReason, bytes int) {
	if c.opt.OnEvict != nil {
		c.opt.OnEvict(key, reason, bytes)
	}
}
