/*
Copyright 2026 Scality.

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

package utils

import (
	"encoding/binary"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2" // nolint:revive,staticcheck
)

const (
	defaultKindBinary  = "kind"
	defaultKindCluster = "kind"
	rfc1918Private     = "172.16.0.0/12"
)

// Run executes the provided command within this context
func Run(cmd *exec.Cmd) (string, error) {
	dir, err := GetProjectDir()
	if err != nil {
		return "", err
	}
	cmd.Dir = dir

	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	command := strings.Join(cmd.Args, " ")
	_, _ = fmt.Fprintf(GinkgoWriter, "running: %q\n", command)
	output, err := cmd.Output()
	if err != nil {
		return string(output), fmt.Errorf("%q failed with error %q: %w", command, string(output), err)
	}

	return string(output), nil
}

// LoadImageToKindClusterWithName loads a local docker image to the kind cluster
func LoadImageToKindClusterWithName(name string) error {
	cluster := defaultKindCluster
	if v, ok := os.LookupEnv("KIND_CLUSTER"); ok {
		cluster = v
	}
	kindOptions := []string{"load", "docker-image", name, "--name", cluster}
	kindBinary := defaultKindBinary
	if v, ok := os.LookupEnv("KIND"); ok {
		kindBinary = v
	}
	cmd := exec.Command(kindBinary, kindOptions...)
	_, err := Run(cmd)
	return err
}

// GetNonEmptyLines converts given command output string into individual objects
// according to line breakers, and ignores the empty elements in it.
func GetNonEmptyLines(output string) []string {
	var res []string
	elements := strings.SplitSeq(output, "\n")
	for element := range elements {
		if element != "" {
			res = append(res, element)
		}
	}

	return res
}

// GetProjectDir returns the project root, i.e. the closest ancestor of the
// working directory holding a go.mod. Tests run from their own package
// directory, but the commands they shell out to (make, kustomize) expect the
// root.
func GetProjectDir() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return wd, fmt.Errorf("failed to get current working directory: %w", err)
	}

	for dir := wd; ; {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return wd, fmt.Errorf("failed to find the project root (no go.mod above %q)", wd)
		}
		dir = parent
	}
}

// GetKindNetwork returns the Docker network a Kind node is attached to, and that network's
// IPv4 subnet. Kind lets Docker choose the subnet -- and the network's name is itself
// overridable (KIND_EXPERIMENTAL_DOCKER_NETWORK) -- so both are read back from a node
// container rather than assumed.
func GetKindNetwork(node string) (string, *net.IPNet, error) {
	output, err := Run(exec.Command("docker", "inspect", node, "-f",
		`{{range $net, $cfg := .NetworkSettings.Networks}}{{if $cfg.IPAddress}}{{$net}} {{end}}{{end}}`))
	if err != nil {
		return "", nil, err
	}
	networks := strings.Fields(output)
	if len(networks) != 1 {
		return "", nil, fmt.Errorf("node %s is attached to %d IPv4 networks, want exactly one", node, len(networks))
	}
	network := networks[0]

	output, err = Run(exec.Command("docker", "network", "inspect", network, "-f",
		`{{range .IPAM.Config}}{{.Subnet}} {{end}}`))
	if err != nil {
		return "", nil, err
	}
	// A Kind network is dual-stack, so keep the IPv4 entry: the operator manages IPv4 only.
	for _, candidate := range strings.Fields(output) {
		ip, subnet, err := net.ParseCIDR(candidate)
		if err != nil {
			return "", nil, fmt.Errorf("parsing subnet %q of network %s: %w", candidate, network, err)
		}
		if ip.To4() != nil {
			return network, subnet, nil
		}
	}
	return "", nil, fmt.Errorf("network %s has no IPv4 subnet", network)
}

func CreateSecondaryNetwork(secondarySubnetName string) (*net.IPNet, error) {
	secondarySubnet, err := unusedPrivateSubnet()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command("docker", "network", "create",
		"--subnet", secondarySubnet.String(),
		secondarySubnetName)
	_, err = Run(cmd)
	if err != nil {
		return nil, err
	}
	return secondarySubnet, nil
}

// ConnectNodeToNetwork attaches a Kind node's container to an extra Docker network, giving
// the node a host interface in that network's subnet. Kind cannot declare extra networks
// itself, and the manager resolves each VirtualIP to an interface of the host network
// namespace -- without this attach, an address on a secondary network resolves to nothing.
// The address is left to Docker's IPAM, which allocates from the bottom of the subnet.
func ConnectNodeToNetwork(node, network string) error {
	_, err := Run(exec.Command("docker", "network", "connect", network, node))
	return err
}

// RemoveNetworkIfPresent makes sure a Docker network is gone, whether it was created by
// this run or left behind by a crashed one. Docker refuses to remove a network that still
// has endpoints -- exactly the state a crashed run leaves, with the Kind nodes still
// attached -- so every attached container is force-disconnected first. The endpoints are
// read back from the network rather than assumed to be the Kind nodes: a stale network may
// hold containers this suite does not know about.
func RemoveNetworkIfPresent(network string) error {
	containers, err := Run(exec.Command("docker", "network", "inspect", network, "-f",
		`{{range .Containers}}{{.Name}} {{end}}`))
	if err != nil {
		// The only expected failure is "no such network", which is the desired state.
		return nil
	}

	for container := range strings.FieldsSeq(containers) {
		if _, err := Run(exec.Command("docker", "network", "disconnect",
			"--force", network, container)); err != nil {
			return err
		}
	}

	_, err = Run(exec.Command("docker", "network", "rm", network))
	return err
}

// unusedPrivateSubnet returns a /16 inside 172.16.0.0/12 that no existing Docker
// network already occupies. Kind (and Docker's default IPAM) also draw from this
// range, so the suite cannot assume a fixed subnet is free.
func unusedPrivateSubnet() (*net.IPNet, error) {
	occupied, err := dockerIPv4Subnets()
	if err != nil {
		return nil, err
	}

	_, private, err := net.ParseCIDR(rfc1918Private)
	if err != nil {
		return nil, err
	}

	base := binary.BigEndian.Uint32(private.IP.To4())
	const prefix = 16
	ones, bits := private.Mask.Size()
	count := uint32(1) << (prefix - ones)
	stride := uint32(1) << (bits - prefix)
	for i := uint32(0); i < count; i++ {
		addr := make(net.IP, net.IPv4len)
		binary.BigEndian.PutUint32(addr, base+i*stride)
		candidate := &net.IPNet{IP: addr, Mask: net.CIDRMask(prefix, 32)}
		if !overlapsAny(candidate, occupied) {
			return candidate, nil
		}
	}
	return nil, fmt.Errorf("no unused /%d in %s", prefix, rfc1918Private)
}

// dockerIPv4Subnets lists the IPv4 subnets of every Docker network. Dual-stack
// networks contribute their IPv4 entry only; networks without IPAM (host, none)
// contribute nothing.
func dockerIPv4Subnets() ([]*net.IPNet, error) {
	ids, err := Run(exec.Command("docker", "network", "ls", "-q"))
	if err != nil {
		return nil, err
	}
	idList := strings.Fields(ids)
	if len(idList) == 0 {
		return nil, nil
	}

	args := append([]string{"network", "inspect", "-f",
		`{{range .IPAM.Config}}{{.Subnet}} {{end}}`}, idList...)
	cmd := exec.Command("docker", args...)
	output, err := Run(cmd)
	if err != nil {
		return nil, err
	}

	subnets := []*net.IPNet{}
	for _, candidate := range strings.Fields(output) {
		ip, subnet, err := net.ParseCIDR(candidate)
		if err != nil {
			return nil, fmt.Errorf("parsing Docker subnet %q: %w", candidate, err)
		}
		if ip.To4() != nil {
			subnets = append(subnets, subnet)
		}
	}
	return subnets, nil
}

func overlapsAny(candidate *net.IPNet, occupied []*net.IPNet) bool {
	for _, existing := range occupied {
		if existing.Contains(candidate.IP) || candidate.Contains(existing.IP) {
			return true
		}
	}
	return false
}
