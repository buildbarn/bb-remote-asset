package qualifier

import (
	"fmt"
	"strings"

	remoteasset "github.com/bazelbuild/remote-apis/build/bazel/remote/asset/v1"
	remoteexecution "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
)

func makeMap(qualifiers []*remoteasset.Qualifier) map[string]string {
	qual := make(map[string]string)
	for _, q := range qualifiers {
		qual[q.GetName()] = q.GetValue()
	}

	return qual
}

// QualifiersToCommand takes a slice of remote asset API qualifiers and
// returns a function which takes a URI and returns a REv2 Command to
// fetch the given URI
func QualifiersToCommand(qArr []*remoteasset.Qualifier) (func(string) *remoteexecution.Command, error) {
	qualifiers := makeMap(qArr)
	resourceType, ok := qualifiers["resource_type"]
	if !ok {
		return nil, fmt.Errorf("missing resource_type")
	}

	switch resourceType {
	case "application/x-git":
		return gitCommand(qualifiers), nil
	case "application/octet-stream":
		return octetStreamCommand(qualifiers), nil
	}

	return nil, fmt.Errorf("unhandled resource_type")
}

// Fetches an asset from a given git repo. Supported qualifiers:
// - vcs.branch: The branch to use
// - vsc.commit: The specific commit
//
// Note that supplying both is valid, however only if the
// requested commit exists on the branch.
func gitCommand(qualifiers map[string]string) func(string) *remoteexecution.Command {
	return func(url string) *remoteexecution.Command {
		script := fmt.Sprintf("git clone %s out", url)
		if branch, ok := qualifiers["vcs.branch"]; ok {
			script = fmt.Sprintf("%s --single-branch --branch %s", script, branch)
		}
		if commit, ok := qualifiers["vcs.commit"]; ok {
			script = fmt.Sprintf("%s && git -C out checkout %s", script, commit)
		}
		return &remoteexecution.Command{
			Arguments:             []string{"sh", "-c", script},
			OutputPaths:           []string{"out"},
			OutputDirectoryFormat: remoteexecution.Command_TREE_AND_DIRECTORY,
		}
	}
}

// Fetches an asset from a given url. Supported qualifiers:
// - auth.basic.username: authentication with a basic username
// - auth.basic.password: authentication with a basic password
// - checksum.sri: verify the checksum after downloading
func octetStreamCommand(qualifiers map[string]string) func(string) *remoteexecution.Command {
	return func(url string) *remoteexecution.Command {
		script := fmt.Sprintf("wget -O out %s", url)
		if username, ok := qualifiers["auth.basic.username"]; ok {
			script = fmt.Sprintf("%s --http-user=%s", script, username)
		}
		if password, ok := qualifiers["auth.basic.password"]; ok {
			script = fmt.Sprintf("%s --http-password=%s", script, password)
		}
		if checksum, ok := qualifiers["checksum.sri"]; ok {
			protocol, base64 := parseChecksum(checksum)
			script = fmt.Sprintf("%s && openssl dgst -%s -binary out | openssl base64 -A | grep %s", script, protocol, base64)
		}

		return &remoteexecution.Command{
			Arguments:             []string{"sh", "-c", script},
			OutputPaths:           []string{"out"},
			OutputDirectoryFormat: remoteexecution.Command_TREE_AND_DIRECTORY,
		}
	}
}

func parseChecksum(c string) (string, string) {
	parts := strings.Split(c, "-")
	return parts[0], parts[1]
}
