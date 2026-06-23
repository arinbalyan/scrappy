package types

import "sync"

type DedupStore struct{ m sync.Map }

func NewDedupStore() *DedupStore { return &DedupStore{} }

func (d *DedupStore) Seen(k string) bool {
	_, ok := d.m.Load(k)
	return ok
}
func (d *DedupStore) Mark(k string) { d.m.Store(k, struct{}{}) }
