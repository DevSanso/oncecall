package pool

import (
	"context"
	"errors"
	"io"
	"oncecall/extension/log"
	"oncecall/extension/metric"
	"sync"
	"testing"
	"time"
)

func newTestPool(t *testing.T, ctx context.Context, m metric.MetricExtension) PoolExtension[*io.PipeReader, int] {
	t.Helper()

	gen := func(_ context.Context, v log.LoggerDebugExtension[any]) (*io.PipeReader, error) {
		reader, writer := io.Pipe()
		_ = writer.Close()
		return reader, nil
	}

	var pool PoolExtension[*io.PipeReader, int] = &TimeoutPoolExtension[*io.PipeReader, int]{
		Name:        "Test",
		Gen:         gen,
		Max:         10,
		Metric:      m,
		Logger:      &log.NoLogLoggerExtension{},
		Context:     ctx,
		IdleTimeout: 10 * time.Second,
	}

	return pool
}

func TestPoolExtensionMax(t *testing.T) {
	m := &metric.SimpleMetricExtension{}
	pool := newTestPool(t, context.Background(), m)
	defer pool.Close()

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := pool.Use(func(data *io.PipeReader) (int, error) {
				time.Sleep(100 * time.Millisecond)
				return 1, nil
			})
			if err != nil {
				t.Errorf("Use() error: %v", err)
			}
		}()
	}
	wg.Wait()

	if got := m.Get("Test", poolExtensionImplMetricFullKey); got > 10 {
		t.Errorf("full = %v, want <= 10", got)
	}
}

func TestPoolExtensionMetric(t *testing.T) {
	m := &metric.SimpleMetricExtension{}
	pool := newTestPool(t, context.Background(), m)
	defer pool.Close()

	_, err := pool.Use(func(data *io.PipeReader) (int, error) {
		return 1, nil
	})
	if err != nil {
		t.Fatalf("Use() error: %v", err)
	}

	if got := m.Get("Test", poolExtensionImplMetricFullKey); got <= 0 {
		t.Errorf("full = %v, want > 0", got)
	}

	if got := m.Get("Test", poolExtensionImplMetricAccErrKey); got != 0 {
		t.Errorf("accErr = %v, want 0", got)
	}

	if got := m.Print(); got == "" {
		t.Error("Print() returned empty string")
	}
}

func TestPoolExtensionIdle(t *testing.T) {
	m := &metric.SimpleMetricExtension{}
	pool := newTestPool(t, context.Background(), m)
	defer pool.Close()

	_, err := pool.Use(func(data *io.PipeReader) (int, error) {
		return 1, nil
	})
	if err != nil {
		t.Fatalf("Use() error: %v", err)
	}

	if got := m.Get("Test", poolExtensionImplMetricIdleKey); got != 1 {
		t.Errorf("idle = %v, want 1", got)
	}
}

func TestPoolExtensionNew(t *testing.T) {
	m := &metric.SimpleMetricExtension{}
	pool := newTestPool(t, context.Background(), m)
	defer pool.Close()
	wg := &sync.WaitGroup{}
	wg1 := &sync.WaitGroup{}
	wg1.Add(1)
	wg.Go(func() {
		_, err := pool.Use(func(data *io.PipeReader) (int, error) {
			wg1.Done()
			time.Sleep(10 * time.Second)
			return 1, nil
		})
		if err != nil {
			t.Fatalf("Use() error: %v", err)
		}
	})
	wg1.Wait()
	_, err := pool.Use(func(data *io.PipeReader) (int, error) {
		return 1, nil
	})

	if err != nil {
		t.Fatalf("Use() error: %v", err)
	}

	if got := m.Get("Test", poolExtensionImplMetricIdleKey); got != 1 {
		t.Errorf("idle = %v, want 1", got)
	}
	if got := m.Get("Test", poolExtensionImplMetricUseKey); got != 1 {
		t.Errorf("idle = %v, want 1", got)
	}
	wg.Wait()
	if got := m.Get("Test", poolExtensionImplMetricIdleKey); got != 2 {
		t.Errorf("idle = %v, want 2", got)
	}
}

func TestPoolExtensionIdleTimeoutFree(t *testing.T) {
	m := &metric.SimpleMetricExtension{}
	pool := newTestPool(t, context.Background(), m)
	defer pool.Close()

	_, err := pool.Use(func(data *io.PipeReader) (int, error) {
		return 1, nil
	})
	if err != nil {
		t.Fatalf("Use() error: %v", err)
	}

	time.Sleep(11 * time.Second)

	_, err = pool.Use(func(data *io.PipeReader) (int, error) {
		return 1, nil
	})
	if err != nil {
		t.Fatalf("Use() error: %v", err)
	}

	if got := m.Get("Test", poolExtensionImplMetricIdleKey); got != 1 {
		t.Errorf("idle = %v, want 1", got)
	}
}

func TestPoolExtensionContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := &metric.SimpleMetricExtension{}
	pool := newTestPool(t, ctx, m)
	defer pool.Close()

	cancel()

	_, err := pool.Use(func(data *io.PipeReader) (int, error) {
		return 1, nil
	})
	if err == nil {
		t.Error("Use() expected context cancellation error")
	}
}

func TestPoolExtensionUseError(t *testing.T) {
	m := &metric.SimpleMetricExtension{}
	pool := newTestPool(t, context.Background(), m)
	defer pool.Close()

	expectedErr := errors.New("test error")

	_, err := pool.Use(func(data *io.PipeReader) (int, error) {
		return 0, expectedErr
	})
	if err == nil {
		t.Fatalf("Use() error = %v, want %v", nil, err)
	}

	if got := m.Get("Test", poolExtensionImplMetricAccErrKey); got != 1 {
		t.Errorf("accErr = %v, want 1", got)
	}
}

func TestPoolExtensionClose(t *testing.T) {
	m := &metric.SimpleMetricExtension{}
	pool := newTestPool(t, context.Background(), m)

	if err := pool.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}
