package hypatia

import (
	"github.com/aws/smithy-go/container/private/cache"
	"github.com/aws/smithy-go/container/private/cache/lru"
	"log"
	"sync"
)

type Cache interface {
	Get(string) (string, bool)
	Put(string, string)
}

func NewCache(size int) Cache {
	return &lruStringCache{
		smithy: lru.New(size),
		m:      &sync.RWMutex{},
	}
}

type lruStringCache struct {
	smithy cache.Cache
	m      *sync.RWMutex
}

func (l *lruStringCache) Get(s string) (string, bool) {
	l.m.RLock()
	defer l.m.RUnlock()
	str, ok := l.smithy.Get(s)
	if _, converts := str.(string); !converts {
		log.Println("expected: ", s, "; got ", str)
		return "", false
	}
	return str.(string), ok
}

func (l *lruStringCache) Put(k string, v string) {
	l.m.Lock()
	defer l.m.Unlock()
	l.smithy.Put(k, v)
}
