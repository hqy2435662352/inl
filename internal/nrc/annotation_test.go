package nrc

import "testing"

func TestAnnotationConstants(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{"AnnotationRisk", AnnotationRisk, "inl.dev/risk"},
		{"AnnotationPureGroup", AnnotationPureGroup, "inl.dev/pure-group"},
		{"AnnotationDataType", AnnotationDataType, "inl.dev/datatype"},
		{"AnnotationFunction", AnnotationFunction, "inl.dev/function"},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}
