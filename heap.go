package gocache

import (
	"time"

	cheap "container/heap"
)

type Entry interface {
	Key() string
	Data() interface{}
}

type entry struct {
	expired time.Time
	data    interface{}
	key     string
}

func (e entry) Key() string {
	return e.key
}

func (e entry) Data() interface{} {
	return e.data
}

type heap struct {
	entries []*entry
	indices map[string]int
}

func (h *heap) Len() int {
	return len(h.entries)
}

func (h *heap) Swap(i, j int) {
	h.entries[i], h.entries[j] = h.entries[j], h.entries[i]
	h.indices[h.entries[i].key] = i
	h.indices[h.entries[j].key] = j
}

func (h *heap) Less(i, j int) bool {
	return h.entries[i].expired.After(h.entries[j].expired)
}

func (h *heap) Pop() interface{} {
	n := len(h.entries) - 1
	o := h.entries[n]
	h.entries[n] = nil
	h.entries = h.entries[:n]
	delete(h.indices, o.key)
	return o
}

func (h *heap) Push(x interface{}) {
	n := len(h.entries)
	item := x.(*entry)
	h.entries = append(h.entries, item)
	h.indices[item.key] = n
}

func (h *heap) top() interface{} {
	n := len(h.entries) - 1
	return h.entries[n]
}

func (h *heap) remove(key string) {
	n := len(h.entries) - 1
	if m, ok := h.indices[key]; ok {
		h.entries[n], h.entries[m] = h.entries[m], h.entries[n]
		h.indices[h.entries[m].key] = m
		h.entries[n] = nil
		h.entries = h.entries[:n]
		delete(h.indices, key)
		if m != n {
			cheap.Fix(h, m)
		}
	}
}

func (h *heap) update(item *entry) {
	n := h.indices[item.key]
	cheap.Fix(h, n)
}

func (h *heap) get(key string) Entry {
	return h.entries[h.indices[key]]
}
