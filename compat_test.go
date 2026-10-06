// SPDX-FileCopyrightText: 2026 Comcast Cable Communications Management, LLC
// SPDX-License-Identifier: Apache-2.0

package wrphttp

import (
	"io/fs"
	"os"
	"os/exec"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestCompat runs the tests in the compat module, which check interop with
// wrp-go/v3.  They are in a separate module to keep the v3 dependencies out
// of this one, so `go test ./...` does not run them on its own.
func TestCompat(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping the compat module in short mode")
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("the go command is not available")
	}

	// The test cache only tracks files this process reads, not files read by
	// the go command it runs.  Read the compat module so a change to it
	// reruns this test instead of reusing a cached result.
	root, err := os.OpenRoot("compat")
	require.NoError(t, err)
	defer root.Close()

	err = fs.WalkDir(root.FS(), ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		_, err = root.ReadFile(path)
		return err
	})
	require.NoError(t, err)

	cmd := exec.CommandContext(t.Context(), "go", "test", "./...")
	cmd.Dir = "compat"
	out, err := cmd.CombinedOutput()
	require.NoError(t, err, "compat tests failed:\n%s", out)
}
