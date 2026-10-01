package generic

import (
	"fmt"
	"oncecall/utils/generic"
	"testing"
)

func TestSyncPool(t *testing.T) {
	p := generic.NewGenericSyncPool(func() struct{ data int } {
		return struct{ data int }{data: 111}
	})

	data := p.Get()

	if data.data != 111 {
		t.Errorf("data != 111")
	}
}

func TestChkDeepCopySyncPool(t *testing.T) {
	origin := struct {
		data  string
		data2 int
	}{
		data2: 1,
		data:  fmt.Sprintf("%s", "hello world"),
	}
	p := generic.NewGenericSyncPool(func() struct {
		data  string
		data2 int
	} {
		return origin
	})

	data := p.Get()

	if &data == &origin {
		t.Errorf("is not deep copy")
	}
	fmt.Printf("%p %p\n", &data.data, &origin.data)
	if &data.data == &origin.data {
		t.Errorf("string element is not deep copy")
	}
}

type TestCloser struct {
	data  string
	data2 int
}

func (d *TestCloser) Close() error {
	return nil
}
func TestSyncUsePool(t *testing.T) {

	origin := TestCloser{
		data2: 1,
		data:  fmt.Sprintf("%s", "hello world"),
	}
	p := generic.NewGenericSyncUsePool[*TestCloser, string](func() (*TestCloser, error) {
		return &TestCloser{data: "hello world"}, nil
	})

	data, err := p.Use(func(data *TestCloser) (string, error) {
		return data.data, nil
	})

	if err != nil {
		t.Error(err)
	}

	if data != origin.data {
		t.Errorf("data != origin.data")
	}

	if pErr := p.Close(); pErr != nil {
		t.Error(pErr)
	}

	_, closeErr := p.Use(func(data *TestCloser) (string, error) {
		return data.data, nil
	})

	if closeErr == nil {
		t.Error("close should fail")
	}

}
