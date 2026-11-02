package errlist

import (
	"fmt"
	"io"
	"testing"
)

func TestErr(t *testing.T) {
	firstErr := ErrG.NewError(io.ErrUnexpectedEOF, "%s", "testing")
	fmt.Println(firstErr)

	secondErr := ErrG.NewError(firstErr, "%s", "testing2")
	fmt.Println(secondErr)
}
