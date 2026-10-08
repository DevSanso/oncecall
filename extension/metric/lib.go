package metric

type MetricExtension interface {
	Add(name, key string, value float64)
	Adds(name string, data map[string]float64)
	Reset()
	Print() string
}
