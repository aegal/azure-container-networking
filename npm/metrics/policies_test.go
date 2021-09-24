package metrics

import "testing"

var (
	numPoliciesMetric = &basicMetric{ResetNumPolicies, IncNumPolicies, DecNumPolicies, GetNumPolicies}
	policyExecMetric  = &recordingMetric{RecordPolicyExecTime, GetPolicyExecCount}
)

func TestRecordPolicyExecTime(t *testing.T) {
	testStopAndRecord(t, policyExecMetric)
}

func TestIncNumPolicies(t *testing.T) {
	testIncMetric(t, numPoliciesMetric)
}

func TestDecNumPolicies(t *testing.T) {
	testDecMetric(t, numPoliciesMetric)
}

func TestResetNumPolicies(t *testing.T) {
	testResetMetric(t, numPoliciesMetric)
}
