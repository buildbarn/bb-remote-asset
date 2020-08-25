package qualifier

import remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

// Sorter implements Sort for an array of Qualifiers to get a consistent hash
type Sorter []*remoteasset.Qualifier

func (q Sorter) Len() int {
	return len(q)
}

func (q Sorter) Less(i, j int) bool {
	return q[i].Name < q[j].Name || (q[i].Name == q[j].Name && q[i].Value < q[j].Value)
}

func (q Sorter) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

// ToArray converts a Sorter back to its underlying array of qualifiers
func (q Sorter) ToArray() []*remoteasset.Qualifier {
	out := make([]*remoteasset.Qualifier, len(q))
	for i := 0; i < len(q); i++ {
		out[i] = q[i]
	}
	return out
}
