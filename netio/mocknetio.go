package netio

import (
	"errors"
	"fmt"
	"net"
)

type MockNetIO struct {
	fail           bool
	failAttempt    int
	numTimesCalled int
}

// ErrMockNetIOFail - mock netio error
var ErrMockNetIOFail = errors.New("netio fail")

func NewMockNetIO(fail bool, failAttempt int) *MockNetIO {
	return &MockNetIO{
		fail:        fail,
		failAttempt: failAttempt,
	}
}

func (netshim *MockNetIO) GetNetworkInterfaceByName(name string) (*net.Interface, error) {
	netshim.numTimesCalled++

	if netshim.fail && netshim.failAttempt == netshim.numTimesCalled {
		return nil, fmt.Errorf("%w:%s", ErrMockNetIOFail, name)
	}

	hwAddr, _ := net.ParseMAC("ab:cd:ef:12:34:56")

	return &net.Interface{
		//nolint:gomnd // Dummy MTU
		MTU:          1000,
		Name:         name,
		HardwareAddr: hwAddr,
		//nolint:gomnd // Dummy interface index
		Index: 2,
	}, nil
}

func (netshim *MockNetIO) GetNetworkInterfaceAddrs(iface *net.Interface) ([]net.Addr, error) {
	return []net.Addr{}, nil
}
