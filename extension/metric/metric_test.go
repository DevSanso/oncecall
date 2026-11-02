package metric

import (
	"strconv"
	"strings"
	"testing"
)

func TestMetricAdd(t *testing.T) {
	m := &SimpleMetricExtension{}

	m.Add("cpu", "usage", 10.5)
	m.Add("cpu", "usage", 5.5)

	result := m.Print()

	if !strings.Contains(result, "cpu") {
		t.Errorf("expected metric name 'cpu' in result, got %s", result)
	}

	if !strings.Contains(result, "16") {
		t.Errorf("expected accumulated value 16, got %s", result)
	}
}

func TestMetricAdds(t *testing.T) {
	m := &SimpleMetricExtension{}

	data := map[string]float64{
		"usage":  10,
		"memory": 20,
		"disk":   30,
	}

	m.Adds("system", data)

	result := m.Print()

	for key, value := range data {
		if !strings.Contains(result, key) {
			t.Errorf("expected key %s in result", key)
		}

		if !strings.Contains(result, formatFloat(value)) {
			t.Errorf("expected value %v in result", value)
		}
	}
}

func TestMetricAssignment(t *testing.T) {
	m := &SimpleMetricExtension{}

	m.Assignment("cpu", "usage", 10)
	m.Assignment("cpu", "usage", 20)

	result := m.Print()

	if !strings.Contains(result, "20") {
		t.Errorf("expected assigned value 20, got %s", result)
	}

	if strings.Contains(result, "10") {
		t.Errorf("old value 10 should have been replaced, got %s", result)
	}
}

func TestMetricAssignments(t *testing.T) {
	m := &SimpleMetricExtension{}

	m.Assignments("system", map[string]float64{
		"cpu":    50,
		"memory": 70,
	})

	m.Assignments("system", map[string]float64{
		"cpu": 90,
	})

	result := m.Print()

	if !strings.Contains(result, "90") {
		t.Errorf("expected updated cpu value 90, got %s", result)
	}

	if !strings.Contains(result, "70") {
		t.Errorf("expected memory value 70 to remain, got %s", result)
	}
}

func TestMetricDeleteFromName(t *testing.T) {
	m := &SimpleMetricExtension{}

	m.Adds("system", map[string]float64{
		"cpu":    50,
		"memory": 70,
	})

	m.Adds("network", map[string]float64{
		"traffic": 100,
	})

	m.DeleteFromName("system")

	result := m.Print()

	if strings.Contains(result, "system") {
		t.Errorf("expected system metric to be deleted, got %s", result)
	}

	if !strings.Contains(result, "network") {
		t.Errorf("expected network metric to remain, got %s", result)
	}
}

func TestMetricReset(t *testing.T) {
	m := &SimpleMetricExtension{}

	m.Add("cpu", "usage", 50)
	m.Add("memory", "usage", 70)

	m.Reset()

	result := m.Print()

	if result != "{}" {
		t.Errorf("expected empty result after reset, got %s", result)
	}
}

func TestMetricPrint(t *testing.T) {
	m := &SimpleMetricExtension{}

	m.Add("system", "cpu", 50)
	m.Add("system", "memory", 80)

	result := m.Print()

	if result == "" {
		t.Error("expected non-empty print result")
	}

	if !strings.Contains(result, "system") {
		t.Errorf("expected system in result, got %s", result)
	}

	if !strings.Contains(result, "cpu") {
		t.Errorf("expected cpu in result, got %s", result)
	}

	if !strings.Contains(result, "memory") {
		t.Errorf("expected memory in result, got %s", result)
	}
}

func TestMetricEmptyOperations(t *testing.T) {
	m := &SimpleMetricExtension{}

	m.DeleteFromName("not-exist")
	m.Adds("empty", map[string]float64{})
	m.Assignments("empty", map[string]float64{})
	m.Reset()

	if result := m.Print(); result != "{}" {
		t.Errorf("expected empty result, got %s", result)
	}
}

func formatFloat(value float64) string {
	return strconv.FormatFloat(value, 'f', -1, 64)
}
