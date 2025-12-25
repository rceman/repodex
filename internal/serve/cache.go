package serve

import (
	"sync"

	"github.com/memkit/repodex/internal/config"
	"github.com/memkit/repodex/internal/index"
	"github.com/memkit/repodex/internal/lang"
	"github.com/memkit/repodex/internal/lang/factory"
	"github.com/memkit/repodex/internal/store"
)

// IndexCache holds preloaded index data for reuse within the serve process.
type IndexCache struct {
	mu       sync.Mutex
	loaded   bool
	cfg      config.Config
	cfgBytes []byte
	plugin   lang.LanguagePlugin
	chunks   []index.ChunkEntry
	chunkMap map[uint32]index.ChunkEntry
	terms    map[string]index.TermInfo
	postings []uint32
}

// Load populates the cache if it is not already loaded.
func (c *IndexCache) Load(root string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.loaded {
		return nil
	}

	cfg, cfgBytes, err := config.Load(store.ConfigPath(root))
	if err != nil {
		return err
	}
	plugin, err := factory.FromProjectType(cfg.ProjectType)
	if err != nil {
		return err
	}
	chunks, err := index.LoadChunkEntries(store.ChunksPath(root))
	if err != nil {
		return err
	}
	chunkMap := make(map[uint32]index.ChunkEntry, len(chunks))
	for _, ch := range chunks {
		chunkMap[ch.ChunkID] = ch
	}
	terms, _, err := index.LoadTerms(store.TermsPath(root))
	if err != nil {
		return err
	}
	postings, err := index.LoadPostings(store.PostingsPath(root))
	if err != nil {
		return err
	}

	c.cfg = cfg
	c.cfgBytes = cfgBytes
	c.plugin = plugin
	c.chunks = chunks
	c.chunkMap = chunkMap
	c.terms = terms
	c.postings = postings
	c.loaded = true
	return nil
}

// Invalidate clears the cached data.
func (c *IndexCache) Invalidate() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.loaded = false
	c.cfg = config.Config{}
	c.cfgBytes = nil
	c.plugin = nil
	c.chunks = nil
	c.chunkMap = nil
	c.terms = nil
	c.postings = nil
}

// Get returns cached index components.
func (c *IndexCache) Get() (config.Config, []byte, lang.LanguagePlugin, []index.ChunkEntry, map[uint32]index.ChunkEntry, map[string]index.TermInfo, []uint32) {
	c.mu.Lock()
	defer c.mu.Unlock()

	cfgCopy := c.cfg
	cfgBytesCopy := make([]byte, len(c.cfgBytes))
	copy(cfgBytesCopy, c.cfgBytes)
	chunksCopy := make([]index.ChunkEntry, len(c.chunks))
	copy(chunksCopy, c.chunks)
	chunkMapCopy := make(map[uint32]index.ChunkEntry, len(c.chunkMap))
	for k, v := range c.chunkMap {
		chunkMapCopy[k] = v
	}
	termsCopy := make(map[string]index.TermInfo, len(c.terms))
	for k, v := range c.terms {
		termsCopy[k] = v
	}
	postingsCopy := make([]uint32, len(c.postings))
	copy(postingsCopy, c.postings)

	return cfgCopy, cfgBytesCopy, c.plugin, chunksCopy, chunkMapCopy, termsCopy, postingsCopy
}
