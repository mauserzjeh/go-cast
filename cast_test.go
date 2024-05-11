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

func TestCastFile(t *testing.T) {
	castFile := New()

	assertEqual(t, castFile.Flags(), 0)
	assertEqual(t, castFile.Version(), 0x1)
	assertEqual(t, len(castFile.Roots()), 0)

	castFile.SetFlags(1).SetVersion(2)
	assertEqual(t, castFile.Flags(), 1)
	assertEqual(t, castFile.Version(), 0x2)
	assertEqual(t, len(castFile.Roots()), 0)

	root := castFile.CreateRoot()
	assertEqual(t, len(castFile.Roots()), 1)
	assertEqual(t, root.Id(), NodeIdRoot)
	assertEqual(t, root.Hash(), castHashBase-1)
	assertEqual(t, root.GetParentNode(), nil)
	assertEqual(t, len(root.GetProperties()), 0)

	mesh := root.CreateChild(NodeIdMesh)
	assertEqual(t, len(root.GetChildNodes()), 1)
	assertEqual(t, root.GetChildNodes()[0], mesh)
	assertEqual(t, root.GetChildrenOfType(NodeIdMesh)[0], mesh)
	assertEqual(t, root.GetChildByHash(mesh.Hash()), mesh)

	prop, err := mesh.CreateProperty(PropString, PropNameName)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, prop.Id(), PropString)
	assertEqual(t, prop.Name(), PropNameName)
	assertEqual(t, prop.Count(), 0)

	p, ok := prop.(*castProperty[string])
	if !ok {
		t.FailNow()
	}
	assertEqual(t, len(p.GetValues()), 0)

	p.AddValues("foo")
	assertEqual(t, len(p.GetValues()), 1)
	assertEqual(t, p.GetValues()[0], "foo")

	prop2, err := CreateProperty(mesh, PropNamePosition, PropVector3, Vec3{
		X: 1,
		Y: 2,
		Z: 3,
	}, Vec3{
		X: 4,
		Y: 5,
		Z: 6,
	})
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, len(mesh.GetProperties()), 2)
	assertEqual(t, prop2.Name(), PropNamePosition)
	assertEqual(t, len(prop2.GetValues()), 2)

	prop2Values, err := GetPropertyValues[Vec3](mesh, PropNamePosition)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, len(prop2Values), 2)
	assertEqual(t, prop2Values[1].Y, 5)

	prop2Value0, err := GetPropertyValue[Vec3](mesh, PropNamePosition)
	if err != nil {
		t.Fatal(err)
	}

	assertEqual(t, prop2Value0.Y, 2)

	_, err = GetPropertyValues[string](mesh, PropNamePosition)
	assertEqual(t, err != nil, true)

	_, err = GetPropertyValues[string](mesh, PropNameEndBone)
	assertEqual(t, err != nil, true)

	_, err = mesh.CreateProperty(PropDouble, PropNameScale)
	if err != nil {
		t.Fatal(err)
	}

	_, err = GetPropertyValue[float64](mesh, PropNameScale)
	assertEqual(t, err != nil, true)

	_, err = mesh.CreateProperty(CastPropertyId(9999), PropNameVertexNormalBuffer)
	assertEqual(t, err != nil, true)
}
