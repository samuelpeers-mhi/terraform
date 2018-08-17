package local

import (
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform/backend"
	"github.com/hashicorp/terraform/providers"
	"github.com/hashicorp/terraform/states/statemgr"
	"github.com/hashicorp/terraform/terraform"
	"github.com/hashicorp/terraform/tfdiags"
)

// TestLocal returns a configured Local struct with temporary paths and
// in-memory ContextOpts.
//
// No operations will be called on the returned value, so you can still set
// public fields without any locks.
func TestLocal(t *testing.T) (*Local, func()) {
	t.Helper()

	tempDir := testTempDir(t)
	var local *Local
	local = &Local{
		StatePath:         filepath.Join(tempDir, "state.tfstate"),
		StateOutPath:      filepath.Join(tempDir, "state.tfstate"),
		StateBackupPath:   filepath.Join(tempDir, "state.tfstate.bak"),
		StateWorkspaceDir: filepath.Join(tempDir, "state.tfstate.d"),
		ContextOpts:       &terraform.ContextOpts{},
		ShowDiagnostics: func(vals ...interface{}) {
			var diags tfdiags.Diagnostics
			diags = diags.Append(vals...)
			for _, diag := range diags {
				t.Log(diag.Description().Summary)
				if local.CLI != nil {
					local.CLI.Error(diag.Description().Summary)
				}
			}
		},
	}
	cleanup := func() {
		if err := os.RemoveAll(tempDir); err != nil {
			t.Fatal("error clecanup up test:", err)
		}
	}

	return local, cleanup
}

// TestLocalProvider modifies the ContextOpts of the *Local parameter to
// have a provider with the given name.
func TestLocalProvider(t *testing.T, b *Local, name string, schema *terraform.ProviderSchema) *terraform.MockResourceProvider {
	// Build a mock resource provider for in-memory operations
	p := new(terraform.MockResourceProvider)
	p.GetSchemaReturn = schema
	p.DiffReturn = &terraform.InstanceDiff{}
	p.RefreshFn = func(
		info *terraform.InstanceInfo,
		s *terraform.InstanceState) (*terraform.InstanceState, error) {
		return s, nil
	}
	p.ResourcesReturn = []terraform.ResourceType{
		terraform.ResourceType{
			Name: "test_instance",
		},
	}

	// Initialize the opts
	if b.ContextOpts == nil {
		b.ContextOpts = &terraform.ContextOpts{}
	}

	// Setup our provider
	b.ContextOpts.ProviderResolver = providers.ResolverFixed(
		map[string]providers.Factory{
			name: providers.FactoryFixed(p),
		},
	)

	return p
}

// TestNewLocalSingle is a factory for creating a TestLocalSingleState.
// This function matches the signature required for backend/init.
func TestNewLocalSingle() backend.Backend {
	return &TestLocalSingleState{}
}

// TestLocalSingleState is a backend implementation that wraps Local
// and modifies it to only support single states (returns
// ErrNamedStatesNotSupported for multi-state operations).
//
// This isn't an actual use case, this is exported just to provide a
// easy way to test that behavior.
type TestLocalSingleState struct {
	Local
}

func (b *TestLocalSingleState) State(name string) (statemgr.Full, error) {
	if name != backend.DefaultStateName {
		return nil, backend.ErrNamedStatesNotSupported
	}

	return b.Local.StateMgr(name)
}

func (b *TestLocalSingleState) States() ([]string, error) {
	return nil, backend.ErrNamedStatesNotSupported
}

func (b *TestLocalSingleState) DeleteState(string) error {
	return backend.ErrNamedStatesNotSupported
}

func testTempDir(t *testing.T) string {
	d, err := ioutil.TempDir("", "tf")
	if err != nil {
		t.Fatalf("err: %s", err)
	}

	return d
}
