package network

import (
	"fmt"
	"net"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/netlink"
	"github.com/Azure/azure-container-networking/network/epcommon"
	"github.com/Azure/azure-container-networking/platform"
)

const (
	virtualGwIPString = "169.254.1.1/32"
	defaultGwCidr     = "0.0.0.0/0"
	defaultGw         = "0.0.0.0"
	virtualv6GwString = "fe80::1234:5678:9abc/128"
	defaultv6Cidr     = "::/0"
	ipv4Bits          = 32
	ipv6Bits          = 128
	ipv4FullMask      = 32
	ipv6FullMask      = 128
)

type TransparentEndpointClient struct {
	bridgeName        string
	hostPrimaryIfName string
	hostVethName      string
	containerVethName string
	hostPrimaryMac    net.HardwareAddr
	containerMac      net.HardwareAddr
	hostVethMac       net.HardwareAddr
	mode              string
}

func NewTransparentEndpointClient(
	extIf *externalInterface,
	hostVethName string,
	containerVethName string,
	mode string,
) *TransparentEndpointClient {

	client := &TransparentEndpointClient{
		bridgeName:        extIf.BridgeName,
		hostPrimaryIfName: extIf.Name,
		hostVethName:      hostVethName,
		containerVethName: containerVethName,
		hostPrimaryMac:    extIf.MacAddress,
		mode:              mode,
	}

	return client
}

func setArpProxy(ifName string) error {
	cmd := fmt.Sprintf("echo 1 > /proc/sys/net/ipv4/conf/%v/proxy_arp", ifName)
	_, err := platform.ExecuteCommand(cmd)
	return err
}

func (client *TransparentEndpointClient) AddEndpoints(epInfo *EndpointInfo) error {
	if _, err := net.InterfaceByName(client.hostVethName); err == nil {
		log.Printf("Deleting old host veth %v", client.hostVethName)
		if err = netlink.DeleteLink(client.hostVethName); err != nil {
			log.Printf("[net] Failed to delete old hostveth %v: %v.", client.hostVethName, err)
			return err
		}
	}

	if err := epcommon.CreateEndpoint(client.hostVethName, client.containerVethName); err != nil {
		return err
	}

	containerIf, err := net.InterfaceByName(client.containerVethName)
	if err != nil {
		return err
	}

	client.containerMac = containerIf.HardwareAddr

	hostVethIf, err := net.InterfaceByName(client.hostVethName)
	if err != nil {
		return err
	}

	client.hostVethMac = hostVethIf.HardwareAddr

	return nil
}

func (client *TransparentEndpointClient) AddEndpointRules(epInfo *EndpointInfo) error {
	var routeInfoList []RouteInfo

	// ip route add <podip> dev <hostveth>
	// This route is needed for incoming packets to pod to route via hostveth
	for _, ipAddr := range epInfo.IPAddresses {
		var (
			routeInfo RouteInfo
			ipNet     net.IPNet
		)

		if ipAddr.IP.To4() != nil {
			ipNet = net.IPNet{IP: ipAddr.IP, Mask: net.CIDRMask(ipv4FullMask, ipv4Bits)}
		} else {
			ipNet = net.IPNet{IP: ipAddr.IP, Mask: net.CIDRMask(ipv6FullMask, ipv6Bits)}
		}
		log.Printf("[net] Adding route for the ip %v", ipNet.String())
		routeInfo.Dst = ipNet
		routeInfoList = append(routeInfoList, routeInfo)
		if err := addRoutes(client.hostVethName, routeInfoList); err != nil {
			return err
		}
	}

	log.Printf("calling setArpProxy for %v", client.hostVethName)
	if err := setArpProxy(client.hostVethName); err != nil {
		log.Printf("setArpProxy failed with: %v", err)
		return err
	}

	return nil
}

func (client *TransparentEndpointClient) DeleteEndpointRules(ep *endpoint) {
	// ip route del <podip> dev <hostveth>
	// Deleting the route set up for routing the incoming packets to pod
	for _, ipAddr := range ep.IPAddresses {
		var (
			routeInfo RouteInfo
			ipNet     net.IPNet
		)

		if ipAddr.IP.To4() != nil {
			ipNet = net.IPNet{IP: ipAddr.IP, Mask: net.CIDRMask(ipv4FullMask, ipv4Bits)}
		} else {
			ipNet = net.IPNet{IP: ipAddr.IP, Mask: net.CIDRMask(ipv6FullMask, ipv6Bits)}
		}

		log.Printf("[net] Deleting route for the ip %v", ipNet.String())
		routeInfo.Dst = ipNet
		if err := deleteRoutes(client.hostVethName, []RouteInfo{routeInfo}); err != nil {
			log.Printf("Error removing route from vm:%+v", err)
		}
	}
}

func (client *TransparentEndpointClient) MoveEndpointsToContainerNS(epInfo *EndpointInfo, nsID uintptr) error {
	// Move the container interface to container's network namespace.
	log.Printf("[net] Setting link %v netns %v.", client.containerVethName, epInfo.NetNsPath)
	if err := netlink.SetLinkNetNs(client.containerVethName, nsID); err != nil {
		return err
	}

	return nil
}

func (client *TransparentEndpointClient) SetupContainerInterfaces(epInfo *EndpointInfo) error {
	if err := epcommon.SetupContainerInterface(client.containerVethName, epInfo.IfName); err != nil {
		return err
	}

	client.containerVethName = epInfo.IfName

	return nil
}

func (client *TransparentEndpointClient) ConfigureContainerInterfacesAndRoutes(epInfo *EndpointInfo) error {
	if err := epcommon.AssignIPToInterface(client.containerVethName, epInfo.IPAddresses); err != nil {
		return err
	}

	// ip route del 10.240.0.0/12 dev eth0 (removing kernel subnet route added by above call)
	for _, ipAddr := range epInfo.IPAddresses {
		_, ipnet, _ := net.ParseCIDR(ipAddr.String())
		routeInfo := RouteInfo{
			Dst:      *ipnet,
			Scope:    netlink.RT_SCOPE_LINK,
			Protocol: netlink.RTPROT_KERNEL,
		}
		if err := deleteRoutes(client.containerVethName, []RouteInfo{routeInfo}); err != nil {
			return err
		}
	}

	// add route for virtualgwip
	// ip route add 169.254.1.1/32 dev eth0
	virtualGwIP, virtualGwNet, _ := net.ParseCIDR(virtualGwIPString)
	routeInfo := RouteInfo{
		Dst:   *virtualGwNet,
		Scope: netlink.RT_SCOPE_LINK,
	}
	if err := addRoutes(client.containerVethName, []RouteInfo{routeInfo}); err != nil {
		return err
	}

	// ip route add default via 169.254.1.1 dev eth0
	_, defaultIPNet, _ := net.ParseCIDR(defaultGwCidr)
	dstIP := net.IPNet{IP: net.ParseIP(defaultGw), Mask: defaultIPNet.Mask}
	routeInfo = RouteInfo{
		Dst: dstIP,
		Gw:  virtualGwIP,
	}
	if err := addRoutes(client.containerVethName, []RouteInfo{routeInfo}); err != nil {
		return err
	}

	// arp -s 169.254.1.1 e3:45:f4:ac:34:12 - add static arp entry for virtualgwip to hostveth interface mac
	log.Printf("[net] Adding static arp for IP address %v and MAC %v in Container namespace",
		virtualGwNet.String(), client.hostVethMac)
	if err := netlink.AddOrRemoveStaticArp(netlink.ADD,
		client.containerVethName,
		virtualGwNet.IP,
		client.hostVethMac,
		false); err != nil {
		return fmt.Errorf("Adding arp in container failed: %w", err)
	}

	if err := client.setupIPV6Routes(epInfo); err != nil {
		return err
	}

	return client.setIPV6NeighEntry(epInfo)
}

func (client *TransparentEndpointClient) setupIPV6Routes(epInfo *EndpointInfo) error {
	if epInfo.IPV6Mode != "" {
		log.Printf("Setting up ipv6 routes in container")

		// add route for virtualgwip
		// ip -6 route add fe80::1234:5678:9abc/128 dev eth0
		virtualGwIP, virtualGwNet, _ := net.ParseCIDR(virtualv6GwString)
		gwRoute := RouteInfo{
			Dst:   *virtualGwNet,
			Scope: netlink.RT_SCOPE_LINK,
		}

		// ip -6 route add default via fe80::1234:5678:9abc dev eth0
		_, defaultIPNet, _ := net.ParseCIDR(defaultv6Cidr)
		log.Printf("defaultv6ipnet :%+v", defaultIPNet)
		defaultRoute := RouteInfo{
			Dst: *defaultIPNet,
			Gw:  virtualGwIP,
		}

		if err := addRoutes(client.containerVethName, []RouteInfo{gwRoute, defaultRoute}); err != nil {
			return err
		}

	}

	return nil
}

func (client *TransparentEndpointClient) setIPV6NeighEntry(epInfo *EndpointInfo) error {
	if epInfo.IPV6Mode != "" {
		log.Printf("[net] Add v6 neigh entry for default gw ip")
		hostGwIP, _, _ := net.ParseCIDR(virtualv6GwString)
		if err := netlink.AddOrRemoveStaticArp(netlink.ADD, client.containerVethName,
			hostGwIP, client.hostVethMac, false); err != nil {
			log.Printf("Failed setting neigh entry in container: %+v", err)
			return fmt.Errorf("Failed setting neigh entry in container: %w", err)
		}
	}

	return nil
}

func (client *TransparentEndpointClient) DeleteEndpoints(ep *endpoint) error {
	return nil
}
