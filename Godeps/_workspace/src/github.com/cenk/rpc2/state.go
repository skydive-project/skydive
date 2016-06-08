package rpc2

import "sync"

type State struct {
	store map[string]interface{}
	m     sync.RWMutex
}

func NewState() *State {
	return &State{store: make(map[string]interface{})}
}

func (s *State) Get(key string) (value interface{}, ok bool) {
	s.m.RLock()
	value, ok = s.store[key]
	s.m.RUnlock()
	return
}

func (s *State) Set(key string, value interface{}) {
	s.m.Lock()
	s.store[key] = value
	s.m.Unlock()
}
