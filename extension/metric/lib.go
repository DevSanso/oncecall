package metric

type MetricSetExtension interface {
	Add(name, key string, value float64)
	Adds(name string, data map[string]float64)
	Assignment(name, key string, value float64)
	Assignments(name string, data map[string]float64)
}

type MetricDropExtension interface {
	DeleteFromName(name string)
}

type MetricExtension interface {
	MetricSetExtension
	MetricDropExtension
	Reset()
	Print() string
}
