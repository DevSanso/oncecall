package err

import (
	"errors"
	"fmt"
	"oncecall/err"
	"testing"
)

func TestErr(t *testing.T) {
	if e := err.Init(); e != nil {
		t.Fatal(e)
		return
	}

	firstErr := err.ErrG.NewError(errors.New("test error"), "%s", "testing")
	fmt.Println(firstErr)

	secondErr := err.ErrG.NewError(firstErr, "%s", "testing2")
	fmt.Println(secondErr)
}
