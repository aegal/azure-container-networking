package policies

type PolicyMap struct {
	cache map[string]*NPMNetworkPolicy
}

type PolicyManager struct {
	policyMap *PolicyMap
}

func NewPolicyManager() *PolicyManager {
	return &PolicyManager{
		policyMap: &PolicyMap{
			cache: make(map[string]*NPMNetworkPolicy),
		},
	}
}

func (pMgr *PolicyManager) PolicyExists(name string) bool {
	_, ok := pMgr.policyMap.cache[name]
	return ok
}

func (pMgr *PolicyManager) GetPolicy(name string) (*NPMNetworkPolicy, bool) {
	policy, ok := pMgr.policyMap.cache[name]
	return policy, ok
}

func (pMgr *PolicyManager) AddPolicy(policy *NPMNetworkPolicy) error {
	// Call actual dataplane function to apply changes
	err := pMgr.addPolicy(policy)
	if err != nil {
		return err
	}

	pMgr.policyMap.cache[policy.Name] = policy
	return nil
}

func (pMgr *PolicyManager) RemovePolicy(name string) error {
	// Call actual dataplane function to apply changes
	err := pMgr.removePolicy(name)
	if err != nil {
		return err
	}

	delete(pMgr.policyMap.cache, name)

	return nil
}

func (pMgr *PolicyManager) UpdatePolicy(policy *NPMNetworkPolicy) error {
	// check and update
	// Call actual dataplane function to apply changes
	err := pMgr.updatePolicy(policy)
	if err != nil {
		return err
	}

	return nil
}
