package rbac

import "testing"

func TestCIGateIntentionalFailure(t *testing.T) {
	t.Fatal("intentional failure to verify the go check blocks PRs")
}
