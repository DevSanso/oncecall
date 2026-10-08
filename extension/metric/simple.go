package metric

import (
	"encoding/json"
	"maps"
	"sync"
)

type SimpleMetricExtension struct {
	m  map[string]map[string]float64
	rw sync.RWMutex
}

func (s *SimpleMetricExtension) DeleteFromName(name string) {
	s.rw.Lock()
	defer s.rw.Unlock()

	subMap, ok := s.m[name]
	if !ok {
		return
	}

	for k := range maps.Keys(subMap) {
		delete(subMap, k)
	}

	delete(s.m, name)
}

// Adds implements MetricExtension.
func (s *SimpleMetricExtension) Adds(name string, data map[string]float64) {
	s.rw.Lock()
	defer s.rw.Unlock()

	if s.m == nil {
		s.m = make(map[string]map[string]float64, 10)
	}

	if _, ok := s.m[name]; !ok {
		s.m[name] = make(map[string]float64, 5)
	}

	for dataK, dataVal := range data {
		if val, ok := s.m[name][dataK]; !ok {
			s.m[name][dataK] = dataVal
		} else {
			s.m[name][dataK] = val + dataVal
		}
	}
}

// Reset implements MetricExtension.
func (s *SimpleMetricExtension) Reset() {
	s.rw.Lock()
	defer s.rw.Unlock()

	for k := range s.m {
		subMap := s.m[k]
		for k := range maps.Keys(subMap) {
			delete(subMap, k)
		}
		delete(s.m, k)
	}

	s.m = nil
}

func (s *SimpleMetricExtension) Add(name, key string, value float64) {
	s.rw.Lock()
	defer s.rw.Unlock()

	if s.m == nil {
		s.m = make(map[string]map[string]float64, 10)
	}

	if _, ok := s.m[name]; !ok {
		s.m[name] = make(map[string]float64, 5)
		
	}

	if val, ok := s.m[name][key]; !ok {
		s.m[name][key] = value
	} else {
		s.m[name][key] = val + value
	}
}

func (s *SimpleMetricExtension) Assignments(name string, data map[string]float64) {
	s.rw.Lock()
	defer s.rw.Unlock()

	if s.m == nil {
		s.m = make(map[string]map[string]float64, 10)
	}

	if _, ok := s.m[name]; !ok {
		s.m[name] = make(map[string]float64, 5)
	}

	maps.Copy(s.m[name], data)
}

func (s *SimpleMetricExtension) Assignment(name, key string, value float64) {
	s.rw.Lock()
	defer s.rw.Unlock()

	if s.m == nil {
		s.m = make(map[string]map[string]float64, 10)
	}

	if _, ok := s.m[name]; !ok {
		s.m[name] = make(map[string]float64, 5)
		
	}

	s.m[name][key] = value
}

func (s *SimpleMetricExtension) Print() string {
	s.rw.RLock()
	defer s.rw.RUnlock()

	if s.m == nil {
		return "{}"
	}

	metricMsg, marshalErr := json.Marshal(s.m)
	if marshalErr != nil {
		panic(marshalErr)
	}

	return string(metricMsg)
}

var _ MetricExtension = (*SimpleMetricExtension)(nil)
