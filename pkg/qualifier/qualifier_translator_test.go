package qualifier_test

import (
	"fmt"
	"testing"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	"github.com/buildbarn/bb-remote-asset/pkg/qualifier"
)

func TestGitCommand(t *testing.T) {
	command, err := qualifier.QualifiersToCommand([]*remoteasset.Qualifier{
		{Name: "resource_type", Value: "application/x-git"},
		{Name: "vcs.branch", Value: "testing"},
	})

	fmt.Print(command("git@github.com:arlyon/graphics.git"), err)
}
