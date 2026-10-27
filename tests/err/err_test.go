package err

import (
	"fmt"
	"io"
	"oncecall/errlist"
	"testing"
)

func TestErr(t *testing.T) {
	firstErr := errlist.ErrG.NewError(io.ErrUnexpectedEOF, "%s", "testing")
	fmt.Println(firstErr)

	secondErr := errlist.ErrG.NewError(firstErr, "%s", "testing2")
	fmt.Println(secondErr)
}
