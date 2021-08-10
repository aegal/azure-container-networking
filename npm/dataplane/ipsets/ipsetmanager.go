package ipsets

import (
	"fmt"
	"net"
	"sync"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/npm/api/v1"
	"github.com/Azure/azure-container-networking/npm/metrics"
	"github.com/Azure/azure-container-networking/npm/util/errors"
)

type IPSetMap struct {
	cache map[string]*api.IPSet
	sync.Mutex
}

func newIPSetMap() *IPSetMap {
	return &IPSetMap{
		cache: make(map[string]*api.IPSet),
	}
}

func (m *IPSetMap) exists(name string) bool {
	_, ok := m.cache[name]
	return ok
}

type IPSetManager struct {
	listMap *IPSetMap
	setMap  *IPSetMap
	os      string
}

func NewIPSetManager() *IPSetManager {
	return &IPSetManager{
		listMap: newIPSetMap(),
		setMap:  newIPSetMap(),
		os:      "linux",
	}
}

func (mgr *IPSetManager) getSetCache(set *api.IPSet) (*IPSetMap, error) {
	kind := getSetKind(set)

	var m *IPSetMap
	switch kind {
	case ListSet:
		m = mgr.listMap
	case HashSet:
		m = mgr.setMap
	default:
		return nil, errors.Errorf(errors.CreateIPSet, false, "unknown Set kind")
	}
	return m, nil
}

// CreateIPSet creates a new ipset of type set or list
func (mgr *IPSetManager) CreateIPSet(set *api.IPSet) error {

	m, err := mgr.getSetCache(set)
	if err != nil {
		return err
	}

	m.Lock()
	defer m.Unlock()
	// Check if the Set already exists
	if m.exists(set.Name) {
		// ipset already exists
		// we should calculate a diff if the members are different
		return nil
	}

	// Call the dataplane specifc fucntion here to
	// create the Set

	// append the cache if dataplane specific function
	// return nil as error
	m.cache[set.Name] = set

	return nil
}

func (mgr *IPSetManager) AddToSet(setName, ip, podKey string) error {

	// check if the IP is IPV$ family
	if net.ParseIP(ip).To4() == nil {
		return errors.Errorf(errors.AppendIPSet, false, "IPV6 not supported")
	}

	mgr.setMap.Lock()
	defer mgr.setMap.Unlock()
	set, exists := mgr.setMap.cache[setName] // check if the Set exists
	if !exists {
		set = NewIPSet(setName, api.SetType_Unknown)
		err := mgr.CreateIPSet(set)
		if err != nil {
			return err
		}
	}

	if getSetKind(set) != HashSet {
		return errors.Errorf(errors.AppendIPSet, false, fmt.Sprintf("ipset %s is not a hash set", setName))
	}
	cachedPodKey, ok := set.IpPodKey[ip]
	if ok {
		if cachedPodKey != podKey {
			log.Logf("AddToSet: PodOwner has changed for Ip: %s, setName:%s, Old podKey: %s, new podKey: %s. Replace context with new PodOwner.",
				ip, setName, cachedPodKey, podKey)

			set.IpPodKey[ip] = podKey
		}
		return nil
	}

	// Now actually add the IP to the Set
	// err := addToSet(setName, ip)
	// some more error handling here

	// update the IP ownership with podkey
	set.IpPodKey[ip] = podKey

	// Update metrics of the IpSet
	metrics.NumIPSetEntries.Inc()
	metrics.IncIPSetInventory(setName)

	return nil
}

func (mgr *IPSetManager) DeleteFromSet(setName, ip, podKey string) error {
	mgr.setMap.Lock()
	defer mgr.setMap.Unlock()
	set, exists := mgr.setMap.cache[setName] // check if the Set exists
	if !exists {
		return errors.Errorf(errors.DeleteIPSet, false, fmt.Sprintf("ipset %s does not exist", setName))
	}

	if getSetKind(set) != HashSet {
		return errors.Errorf(errors.DeleteIPSet, false, fmt.Sprintf("ipset %s is not a hash set", setName))
	}

	// in case the IP belongs to a new Pod, then ignore this Delete call as this might be stale
	cachedPodKey := set.IpPodKey[ip]
	if cachedPodKey != podKey {
		log.Logf("DeleteFromSet: PodOwner has changed for Ip: %s, setName:%s, Old podKey: %s, new podKey: %s. Ignore the delete as this is stale update",
			ip, setName, cachedPodKey, podKey)

		return nil
	}

	// Now actually delete the IP from the Set
	// err := deleteFromSet(setName, ip)
	// some more error handling here

	// update the IP ownership with podkey
	delete(set.IpPodKey, ip)

	// Update metrics of the IpSet
	metrics.NumIPSetEntries.Dec()
	metrics.DecIPSetInventory(setName)

	return nil
}

func (mgr *IPSetManager) AddToList(listName, setName string) error {

	if listName == setName {
		return errors.Errorf(errors.AppendIPSet, false, fmt.Sprintf("list %s cannot be added to itself", listName))
	}

	mgr.listMap.Lock()
	defer mgr.listMap.Unlock()
	set, exists := mgr.setMap.cache[setName] // check if the Set exists
	if !exists {
		return errors.Errorf(errors.AppendIPSet, false, fmt.Sprintf("member ipset %s does not exist", setName))
	}

	// Nested IPSets are only supported for windows
	//Check if we want to actually use that support
	if getSetKind(set) != HashSet && mgr.os != "windows" {
		return errors.Errorf(errors.DeleteIPSet, false, fmt.Sprintf("member ipset %s is not a Set type and nestetd ipsets are not supported", setName))
	}

	list, exists := mgr.listMap.cache[listName] // check if the Set exists
	if !exists {
		return errors.Errorf(errors.AppendIPSet, false, fmt.Sprintf("ipset %s does not exist", listName))
	}

	if getSetKind(list) != ListSet {
		return errors.Errorf(errors.AppendIPSet, false, fmt.Sprintf("ipset %s is not a list set", listName))
	}

	// check if Set is a member of List
	listSet, exists := list.IPSet[setName]
	if exists {
		if listSet == set {
			// Set is already a member of List
			return nil
		}
		// Update the ipset in list
		list.IPSet[setName] = set
		return nil
	}

	// Now actually add the Set to the List
	// err := addToList(listName, setName)
	// some more error handling here

	// update the Ipset member list of list
	list.IPSet[setName] = set

	// Update metrics of the IpSet
	metrics.NumIPSetEntries.Inc()
	metrics.IncIPSetInventory(setName)

	return nil
}

func (mgr *IPSetManager) DeleteFromList(listName, setName string) error {
	mgr.listMap.Lock()
	defer mgr.listMap.Unlock()
	set, exists := mgr.setMap.cache[setName] // check if the Set exists
	if !exists {
		return errors.Errorf(errors.DeleteIPSet, false, fmt.Sprintf("ipset %s does not exist", setName))
	}

	if getSetKind(set) != HashSet {
		return errors.Errorf(errors.DeleteIPSet, false, fmt.Sprintf("ipset %s is not a hash set", setName))
	}

	// Nested IPSets are only supported for windows
	//Check if we want to actually use that support
	if getSetKind(set) != HashSet && mgr.os != "windows" {
		return errors.Errorf(errors.DeleteIPSet, false, fmt.Sprintf("member ipset %s is not a Set type and nestetd ipsets are not supported", setName))
	}

	list, exists := mgr.listMap.cache[listName] // check if the Set exists
	if !exists {
		return errors.Errorf(errors.DeleteIPSet, false, fmt.Sprintf("ipset %s does not exist", listName))
	}

	if getSetKind(list) != ListSet {
		return errors.Errorf(errors.DeleteIPSet, false, fmt.Sprintf("ipset %s is not a list set", listName))
	}

	// check if Set is a member of List
	_, exists = list.IPSet[setName]
	if !exists {
		return nil
	}

	// Now actually delete the Set from the List
	// err := deleteFromList(listName, setName)
	// some more error handling here

	// update the I
	return nil
}

func (mgr *IPSetManager) DeleteList(name string) {
	mgr.listMap.Lock()
	defer mgr.listMap.Unlock()
	delete(mgr.listMap.cache, name)
}

func (mgr *IPSetManager) DeleteSet(name string) {
	mgr.setMap.Lock()
	defer mgr.setMap.Unlock()
	delete(mgr.setMap.cache, name)
}

// TODO: do we need this function ?
func (mgr *IPSetManager) Clear() {
	mgr.listMap.Lock()
	defer mgr.listMap.Unlock()
	mgr.listMap.cache = make(map[string]*api.IPSet)
	mgr.setMap.Lock()
	defer mgr.setMap.Unlock()
	mgr.setMap.cache = make(map[string]*api.IPSet)
}
