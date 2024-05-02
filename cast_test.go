package cast

import (
	"fmt"
	"os"
	"testing"
)

func TestCastFile(t *testing.T) {
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
