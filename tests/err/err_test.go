package err

import (
	"fmt"
	"oncecall/errlist"
	"testing"
)

func TestErr(t *testing.T) {
	if e := errlist.Init(); e != nil {
		t.Fatal(e)
		return
	}

	firstErr := errlist.ErrG.NewError(nil, "%s", "testing")
	fmt.Println(firstErr)

	secondErr := errlist.ErrG.NewError(firstErr, "%s", "testing2")
	fmt.Println(secondErr)
}
