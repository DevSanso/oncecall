package metric

import (
	"crypto/md5"
	"encoding/json"
	"sync"
)

type SimpleMetricExtension struct {
	m  map[[32]byte]mapfloat64
	rw sync.RWMutex
}

func (*SimpleMetricExtension) makeHash(name string, key string) {
	md5.Sum()
}

func (s *SimpleMetricExtension) Add(name, key string, value float64) {
	s.rw.Lock()
	defer s.rw.Unlock()

	if s.m == nil {
		s.m = make(map[string]float64, 5)
	}

	s.m[key] = value
}

func (s *SimpleMetricExtension) Adds(name, data map[string]float64) {
	s.rw.Lock()
	defer s.rw.Unlock()

	if s.m == nil {
		s.m = make(map[string]float64, 5)
	}

	for k := range data {
		s.m[k] = data[k] + s.m[k]
	}
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
