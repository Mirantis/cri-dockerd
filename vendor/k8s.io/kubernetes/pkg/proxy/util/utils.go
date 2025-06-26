/*
Copyright 2017 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"fmt"
	"net"
	"strconv"
	"strings"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	utilrand "k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/sets"
	utilfeature "k8s.io/apiserver/pkg/util/feature"
	"k8s.io/client-go/tools/events"
	utilsysctl "k8s.io/component-helpers/node/util/sysctl"
	"k8s.io/klog/v2"
	helper "k8s.io/kubernetes/pkg/apis/core/v1/helper"
	"k8s.io/kubernetes/pkg/features"
	netutils "k8s.io/utils/net"
)

const (
	// IPv4ZeroCIDR is the CIDR block for the whole IPv4 address space
	IPv4ZeroCIDR = "0.0.0.0/0"

	// IPv6ZeroCIDR is the CIDR block for the whole IPv6 address space
	IPv6ZeroCIDR = "::/0"
)

// isValidEndpoint checks that the given host / port pair are valid endpoint
func isValidEndpoint(host string, port int) bool {
	return host != "" && port > 0
}

// BuildPortsToEndpointsMap builds a map of portname -> all ip:ports for that
// portname. Explode Endpoints.Subsets[*] into this structure.
func BuildPortsToEndpointsMap(endpoints *v1.Endpoints) map[string][]string {
	portsToEndpoints := map[string][]string{}
	for i := range endpoints.Subsets {
		ss := &endpoints.Subsets[i]
		for i := range ss.Ports {
			port := &ss.Ports[i]
			for i := range ss.Addresses {
				addr := &ss.Addresses[i]
				if isValidEndpoint(addr.IP, int(port.Port)) {
					portsToEndpoints[port.Name] = append(portsToEndpoints[port.Name], net.JoinHostPort(addr.IP, strconv.Itoa(int(port.Port))))
				}
			}
		}
	}
	return portsToEndpoints
}

// IsZeroCIDR checks whether the input CIDR string is either
// the IPv4 or IPv6 zero CIDR
func IsZeroCIDR(cidr string) bool {
	if cidr == IPv4ZeroCIDR || cidr == IPv6ZeroCIDR {
		return true
	}
	return false
}

// IsLoopBack checks if a given IP address is a loopback address.
func IsLoopBack(ip string) bool {
	netIP := netutils.ParseIPSloppy(ip)
	if netIP != nil {
		return netIP.IsLoopback()
	}
	return false
}

// GetLocalAddrs returns a list of all network addresses on the local system
func GetLocalAddrs() ([]net.IP, error) {
	var localAddrs []net.IP

	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil, err
	}

	for _, addr := range addrs {
		ip, _, err := netutils.ParseCIDRSloppy(addr.String())
		if err != nil {
			return nil, err
		}

		localAddrs = append(localAddrs, ip)
	}

	return localAddrs, nil
}

// GetLocalAddrSet return a local IPSet.
// If failed to get local addr, will assume no local ips.
func GetLocalAddrSet() netutils.IPSet {
	localAddrs, err := GetLocalAddrs()
	if err != nil {
		klog.ErrorS(err, "Failed to get local addresses assuming no local IPs")
	} else if len(localAddrs) == 0 {
		klog.InfoS("No local addresses were found")
	}

	localAddrSet := netutils.IPSet{}
	localAddrSet.Insert(localAddrs...)
	return localAddrSet
}

// ShouldSkipService checks if a given service should skip proxying
func ShouldSkipService(service *v1.Service) bool {
	// if ClusterIP is "None" or empty, skip proxying
	if !helper.IsServiceIPSet(service) {
		klog.V(3).InfoS("Skipping service due to cluster IP", "service", klog.KObj(service), "clusterIP", service.Spec.ClusterIP)
		return true
	}
	// Even if ClusterIP is set, ServiceTypeExternalName services don't get proxied
	if service.Spec.Type == v1.ServiceTypeExternalName {
		klog.V(3).InfoS("Skipping service due to Type=ExternalName", "service", klog.KObj(service))
		return true
	}
	return false
}

// AddressSet validates the addresses in the slice using the "isValid" function.
// Addresses that pass the validation are returned as a string Set.
func AddressSet(isValid func(ip net.IP) bool, addrs []net.Addr) sets.Set[string] {
	ips := sets.New[string]()
	for _, a := range addrs {
		var ip net.IP
		switch v := a.(type) {
		case *net.IPAddr:
			ip = v.IP
		case *net.IPNet:
			ip = v.IP
		default:
			continue
		}
		if isValid(ip) {
			ips.Insert(ip.String())
		}
	}
	return ips
}

// LogAndEmitIncorrectIPVersionEvent logs and emits incorrect IP version event.
func LogAndEmitIncorrectIPVersionEvent(recorder events.EventRecorder, fieldName, fieldValue, svcNamespace, svcName string, svcUID types.UID) {
	errMsg := fmt.Sprintf("%s in %s has incorrect IP version", fieldValue, fieldName)
	klog.ErrorS(nil, "Incorrect IP version", "service", klog.KRef(svcNamespace, svcName), "field", fieldName, "value", fieldValue)
	if recorder != nil {
		recorder.Eventf(
			&v1.ObjectReference{
				Kind:      "Service",
				Name:      svcName,
				Namespace: svcNamespace,
				UID:       svcUID,
			}, nil, v1.EventTypeWarning, "KubeProxyIncorrectIPVersion", "GatherEndpoints", errMsg)
	}
}

// MapIPsByIPFamily maps a slice of IPs to their respective IP families (v4 or v6)
func MapIPsByIPFamily(ipStrings []string) map[v1.IPFamily][]net.IP {
	ipFamilyMap := map[v1.IPFamily][]net.IP{}
	for _, ipStr := range ipStrings {
		ip := netutils.ParseIPSloppy(ipStr)
		if ip != nil {
			// Since ip is parsed ok, GetIPFamilyFromIP will never return v1.IPFamilyUnknown
			ipFamily := GetIPFamilyFromIP(ip)
			ipFamilyMap[ipFamily] = append(ipFamilyMap[ipFamily], ip)
		} else {
			// ExternalIPs may not be validated by the api-server.
			// Specifically empty strings validation, which yields into a lot
			// of bad error logs.
			if len(strings.TrimSpace(ipStr)) != 0 {
				klog.ErrorS(nil, "Skipping invalid IP", "ip", ipStr)
			}
		}
	}
	return ipFamilyMap
}

// MapCIDRsByIPFamily maps a slice of CIDRs to their respective IP families (v4 or v6)
func MapCIDRsByIPFamily(cidrsStrings []string) map[v1.IPFamily][]*net.IPNet {
	ipFamilyMap := map[v1.IPFamily][]*net.IPNet{}
	for _, cidrStrUntrimmed := range cidrsStrings {
		cidrStr := strings.TrimSpace(cidrStrUntrimmed)
		_, cidr, err := netutils.ParseCIDRSloppy(cidrStr)
		if err != nil {
			// Ignore empty strings. Same as in MapIPsByIPFamily
			if len(cidrStr) != 0 {
				klog.ErrorS(err, "Invalid CIDR ignored", "CIDR", cidrStr)
			}
			continue
		}
		// since we just succefully parsed the CIDR, IPFamilyOfCIDR will never return "IPFamilyUnknown"
		ipFamily := convertToV1IPFamily(netutils.IPFamilyOfCIDR(cidr))
		ipFamilyMap[ipFamily] = append(ipFamilyMap[ipFamily], cidr)
	}
	return ipFamilyMap
}

// GetIPFamilyFromIP Returns the IP family of ipStr, or IPFamilyUnknown if ipStr can't be parsed as an IP
func GetIPFamilyFromIP(ip net.IP) v1.IPFamily {
	return convertToV1IPFamily(netutils.IPFamilyOf(ip))
}

// Convert netutils.IPFamily to v1.IPFamily
func convertToV1IPFamily(ipFamily netutils.IPFamily) v1.IPFamily {
	switch ipFamily {
	case netutils.IPv4:
		return v1.IPv4Protocol
	case netutils.IPv6:
		return v1.IPv6Protocol
	}

	return v1.IPFamilyUnknown
}

// OtherIPFamily returns the other ip family
func OtherIPFamily(ipFamily v1.IPFamily) v1.IPFamily {
	if ipFamily == v1.IPv6Protocol {
		return v1.IPv4Protocol
	}

	return v1.IPv6Protocol
}

// AppendPortIfNeeded appends the given port to IP address unless it is already in
// "ipv4:port" or "[ipv6]:port" format.
func AppendPortIfNeeded(addr string, port int32) string {
	// Return if address is already in "ipv4:port" or "[ipv6]:port" format.
	if _, _, err := net.SplitHostPort(addr); err == nil {
		return addr
	}

	// Simply return for invalid case. This should be caught by validation instead.
	ip := netutils.ParseIPSloppy(addr)
	if ip == nil {
		return addr
	}

	// Append port to address.
	if ip.To4() != nil {
		return fmt.Sprintf("%s:%d", addr, port)
	}
	return fmt.Sprintf("[%s]:%d", addr, port)
}

// ShuffleStrings copies strings from the specified slice into a copy in random
// order. It returns a new slice.
func ShuffleStrings(s []string) []string {
	if s == nil {
		return nil
	}
	shuffled := make([]string, len(s))
	perm := utilrand.Perm(len(s))
	for i, j := range perm {
		shuffled[j] = s[i]
	}
	return shuffled
}

// EnsureSysctl sets a kernel sysctl to a given numeric value.
func EnsureSysctl(sysctl utilsysctl.Interface, name string, newVal int) error {
	if oldVal, _ := sysctl.GetSysctl(name); oldVal != newVal {
		if err := sysctl.SetSysctl(name, newVal); err != nil {
			return fmt.Errorf("can't set sysctl %s to %d: %v", name, newVal, err)
		}
		klog.V(1).InfoS("Changed sysctl", "name", name, "before", oldVal, "after", newVal)
	}
	return nil
}

// GetClusterIPByFamily returns a service clusterip by family
func GetClusterIPByFamily(ipFamily v1.IPFamily, service *v1.Service) string {
	// allowing skew
	if len(service.Spec.IPFamilies) == 0 {
		if len(service.Spec.ClusterIP) == 0 || service.Spec.ClusterIP == v1.ClusterIPNone {
			return ""
		}

		IsIPv6Family := (ipFamily == v1.IPv6Protocol)
		if IsIPv6Family == netutils.IsIPv6String(service.Spec.ClusterIP) {
			return service.Spec.ClusterIP
		}

		return ""
	}

	for idx, family := range service.Spec.IPFamilies {
		if family == ipFamily {
			if idx < len(service.Spec.ClusterIPs) {
				return service.Spec.ClusterIPs[idx]
			}
		}
	}

	return ""
}

// RevertPorts is closing ports in replacementPortsMap but not in originalPortsMap. In other words, it only
// closes the ports opened in this sync.
func RevertPorts(replacementPortsMap, originalPortsMap map[netutils.LocalPort]netutils.Closeable) {
	for k, v := range replacementPortsMap {
		// Only close newly opened local ports - leave ones that were open before this update
		if originalPortsMap[k] == nil {
			klog.V(2).InfoS("Closing local port", "port", k.String())
			v.Close()
		}
	}
}

func IsVIPMode(ing v1.LoadBalancerIngress) bool {
	if !utilfeature.DefaultFeatureGate.Enabled(features.LoadBalancerIPMode) {
		return true // backwards compat
	}
	if ing.IPMode == nil {
		return true
	}
	return *ing.IPMode == v1.LoadBalancerIPModeVIP
}
