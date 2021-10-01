package dataplane

import (
	"testing"

	"github.com/Azure/azure-container-networking/npm/metrics"
	"github.com/Azure/azure-container-networking/npm/pkg/dataplane/ipsets"
)

func TestNewDataPlane(t *testing.T) {
	metrics.InitializeAll()
	dp := NewDataPlane("testnode")

	if dp == nil {
		t.Error("NewDataPlane() returned nil")
	}

	dp.CreateIPSet("test", ipsets.NameSpace)
}
