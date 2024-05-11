package cast

import (
	"fmt"
	"io"
	"os"
	"testing"
)

// assertEqual fails if the two values are not equal
func assertEqual[T comparable](t testing.TB, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("got: %v != want: %v", got, want)
	}
}

func TestLoadCastFile(t *testing.T) {
	for _, f := range []string{
		"cube.cast",
		"cast_constraints.cast",
		"cast_ik.cast",
		"pilot_medium_bangalore_LOD0.cast",
	} {
		r, err := os.Open(fmt.Sprintf("testdata/%v", f))
		if err != nil {
			t.Fatalf("%v", err)
		}
		defer r.Close()

		cast, err := Load(r)
		if err != nil {
			t.Fatalf("%v", err)
		}
		_ = cast
	}
}

func TestWriteCastFile(t *testing.T) {
	for _, f := range []string{
		"cube.cast",
		"cast_constraints.cast",
		"cast_ik.cast",
		"pilot_medium_bangalore_LOD0.cast",
	} {
		r, err := os.Open(fmt.Sprintf("testdata/%v", f))
		if err != nil {
			t.Fatalf("%v", err)
		}
		defer r.Close()

		cast, err := Load(r)
		if err != nil {
			t.Fatalf("%v", err)
		}

		if err := cast.Write(io.Discard); err != nil {
			t.Fatalf("%v", err)
		}
	}
}
