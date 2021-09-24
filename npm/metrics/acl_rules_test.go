package metrics

import "testing"

var (
	numRulesMetric = &basicMetric{ResetNumACLRules, IncNumACLRules, DecNumACLRules, GetNumACLRules}
	ruleExecMetric = &recordingMetric{RecordACLRuleExecTime, GetACLRuleExecCount}
)

func TestRecordACLRuleExecTime(t *testing.T) {
	testStopAndRecord(t, ruleExecMetric)
}

func TestIncNumACLRules(t *testing.T) {
	testIncMetric(t, numRulesMetric)
}

func TestDecNumACLRules(t *testing.T) {
	testDecMetric(t, numRulesMetric)
}

func TestResetNumACLRules(t *testing.T) {
	testResetMetric(t, numRulesMetric)
}
