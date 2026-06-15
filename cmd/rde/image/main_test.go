package image

import (
	"os"
	"testing"

	"github.com/bitrise-io/bitrise-cli/cmd/cmdtest"
)

func TestMain(m *testing.M) { os.Exit(cmdtest.RunIsolated(m)) }
