package ipsets

import (
	"os"
	"testing"

	"github.com/Azure/azure-container-networking/npm/metrics"
)

const (
	testSetName  = "test-set"
	testListName = "test-list"
	testPodKey   = "test-pod-key"
	testPodIP    = "10.0.0.0"
)

func TestCreateIPSet(t *testing.T) {
	iMgr := NewIPSetManager("azure")

	iMgr.CreateIPSet(testSetName, NameSpace)

	// TODO add cache check
}

func TestAddToSet(t *testing.T) {
	iMgr := NewIPSetManager("azure")

	iMgr.CreateIPSet(testSetName, NameSpace)

	err := iMgr.AddToSet([]string{testSetName}, testPodIP, testPodKey)
	if err != nil {
		t.Errorf("AddToSet() returned error %s", err.Error())
	}
}

func TestRemoveFromSet(t *testing.T) {
	iMgr := NewIPSetManager("azure")

	iMgr.CreateIPSet(testSetName, NameSpace)
	err := iMgr.AddToSet([]string{testSetName}, testPodIP, testPodKey)
	if err != nil {
		t.Errorf("RemoveFromSet() returned error %s", err.Error())
	}
	err = iMgr.RemoveFromSet([]string{testSetName}, testPodIP, testPodKey)
	if err != nil {
		t.Errorf("RemoveFromSet() returned error %s", err.Error())
	}
}

func TestRemoveFromSetMissing(t *testing.T) {
	iMgr := NewIPSetManager("azure")
	err := iMgr.RemoveFromSet([]string{testSetName}, testPodIP, testPodKey)
	if err == nil {
		t.Errorf("RemoveFromSet() did not return error")
	}
}

func TestAddToListMissing(t *testing.T) {
	iMgr := NewIPSetManager("azure")
	err := iMgr.AddToList(testPodKey, []string{"newtest"})
	if err == nil {
		t.Errorf("AddToList() did not return error")
	}
}

func TestAddToList(t *testing.T) {
	iMgr := NewIPSetManager("azure")
	iMgr.CreateIPSet(testSetName, NameSpace)
	iMgr.CreateIPSet(testListName, KeyLabelOfNameSpace)

	err := iMgr.AddToList(testListName, []string{testSetName})
	if err != nil {
		t.Errorf("AddToList() returned error %s", err.Error())
	}
}

func TestRemoveFromList(t *testing.T) {
	iMgr := NewIPSetManager("azure")
	iMgr.CreateIPSet(testSetName, NameSpace)
	iMgr.CreateIPSet(testListName, KeyLabelOfNameSpace)

	err := iMgr.AddToList(testListName, []string{testSetName})
	if err != nil {
		t.Errorf("AddToList() returned error %s", err.Error())
	}

	err = iMgr.RemoveFromList(testListName, []string{testSetName})
	if err != nil {
		t.Errorf("RemoveFromList() returned error %s", err.Error())
	}
}

func TestRemoveFromListMissing(t *testing.T) {
	iMgr := NewIPSetManager("azure")

	iMgr.CreateIPSet(testListName, KeyLabelOfNameSpace)

	err := iMgr.RemoveFromList(testListName, []string{testSetName})
	if err == nil {
		t.Errorf("RemoveFromList() did not return error")
	}
}

func TestDeleteIPSet(t *testing.T) {
	iMgr := NewIPSetManager("azure")
	iMgr.CreateIPSet(testSetName, NameSpace)

	iMgr.DeleteIPSet(testSetName)
	// TODO add cache check
}

func TestMain(m *testing.M) {
	metrics.InitializeAll()

	exitCode := m.Run()

	os.Exit(exitCode)
}
