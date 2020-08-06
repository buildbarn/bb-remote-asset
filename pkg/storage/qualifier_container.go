package storage

import remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"

type qualifierContainer []*remoteasset.Qualifier

func (q qualifierContainer) Len() int {
	return len(q)
}

func (q qualifierContainer) Less(i, j int) bool {
	return q[i].Name < q[j].Name || (q[i].Name == q[j].Name && q[i].Value < q[j].Value)
}

func (q qualifierContainer) Swap(i, j int) {
	q[i], q[j] = q[j], q[i]
}

func (q qualifierContainer) toArray() []*remoteasset.Qualifier {
	out := make([]*remoteasset.Qualifier, len(q))
	for i := 0; i < len(q); i++ {
		out[i] = q[i]
	}
	return out
}
