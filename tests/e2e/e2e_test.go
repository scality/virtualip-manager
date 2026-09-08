//go:build e2e
// +build e2e

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

package e2e

import (
	"encoding/binary"
	"fmt"
	"maps"
	"net"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/scality/virtualip-manager/pkg/domain"
	"github.com/scality/virtualip-manager/tests/utils"
)

// namespace where the project is deployed in
const namespace = "virtualip-manager-system"

// virtualIPConfigMapName is the ConfigMap the DaemonSet mounts as its input spec.
// The name is also hardcoded in virtualip-manager.yaml.
const virtualIPConfigMapName = "virtualip-manager-config"

// secondaryNetworkName is the extra Docker network the suite creates so the
// DaemonSet can be exercised on more than Kind's own bridge. Kind's teardown
// does not remove it, so AfterAll does.
const secondaryNetworkName = "secondary-network"

// managerNodeIP is the NODE_IP the DaemonSet advertises. It is hardcoded in
// virtualip-manager.yaml rather than taken from the node so the healthcheck the manager
// renders is predictable.
const managerNodeIP = "1.1.1.1"

// generatedConfigPath is where the entrypoint renders the keepalived configuration before
// handing over to keepalived. The path is hardcoded in scripts/entrypoint.sh.
const generatedConfigPath = "/etc/keepalived/keepalived.conf"

// The capability bits the Dockerfile grants keepalived with setcap. Nothing outside this set
// should end up in the effective set of the running process.
const (
	capSetGid         = 6
	capSetUid         = 7
	capNetBindService = 10
	capNetAdmin       = 12
	capNetRaw         = 13
)

//nolint:gochecknoglobals // A constant expression Go will not let us declare as a constant.
var expectedCapabilities = uint64(1<<capSetGid | 1<<capSetUid | 1<<capNetBindService |
	1<<capNetAdmin | 1<<capNetRaw)

var (
	nodeNames     []string
	nodeName      string
	otherNodeName string

	// lastVrId is the last VRRP virtual_router_id handed out by nextVrId. The ids have to be
	// unique across every address of the spec, not just within one network.
	lastVrId int

	kindNetworkName string
	kindSubnet      *net.IPNet
	secondarySubnet *net.IPNet
)

var _ = Describe("Manager", Ordered, func() {
	// managerPods maps a labelled node to the virtualip-manager pod running on it. The pods
	// cannot be told apart by name -- the DaemonSet generates those -- and which one is MASTER
	// for a given address depends on the node, so every assertion needs the mapping.
	managerPods := map[string]string{}

	// verifyManagerUp waits for one non-terminating, ready virtualip-manager pod per labelled
	// node and records them by node. Every step that restarts the DaemonSet has to wait for
	// this again: the pod names are stale as soon as the rollout starts.
	verifyManagerUp := func(g Gomega) {
		By("getting the virtualip-manager pods and the nodes they run on")
		cmd := exec.Command("kubectl", "get",
			"pods", "-l", "app.kubernetes.io/name=virtualip-manager",
			"-o", "go-template={{ range .items }}"+
				"{{ if not .metadata.deletionTimestamp }}"+
				"{{ .spec.nodeName }}={{ .metadata.name }}"+
				"{{ \"\\n\" }}{{ end }}{{ end }}",
			"-n", namespace,
		)

		podOutput, err := utils.Run(cmd)
		g.Expect(err).NotTo(HaveOccurred(), "Failed to retrieve virtualip-manager pod information")

		pods := map[string]string{}
		for _, line := range utils.GetNonEmptyLines(podOutput) {
			node, pod, found := strings.Cut(line, "=")
			g.Expect(found).To(BeTrue(), "unexpected pod listing %q", line)
			pods[node] = pod
		}
		g.Expect(slices.Sorted(maps.Keys(pods))).To(Equal(slices.Sorted(slices.Values(nodeNames))),
			"expected exactly one virtualip-manager pod per node labelled role=virtualip")

		By("validating the pods' status")
		// The readiness of the container is part of the assertion: a container that
		// generate-config fails leaves the pod in CrashLoopBackOff, whose phase is still
		// Running, so the phase alone would not notice a manager that never starts.
		for node, pod := range pods {
			cmd = exec.Command("kubectl", "get",
				"pods", pod, "-n", namespace,
				"-o", "jsonpath={.status.phase}/{.status.containerStatuses[0].ready}",
			)
			output, err := utils.Run(cmd)
			g.Expect(err).NotTo(HaveOccurred())
			g.Expect(output).To(Equal("Running/true"),
				"Incorrect virtualip-manager pod status on node %s", node)
		}

		clear(managerPods)
		maps.Copy(managerPods, pods)
	}

	// Before running the tests, set up the environment by creating the namespace,
	// enforcing the privileged security policy on it -- keepalived needs
	// hostNetwork and the NET_ADMIN/NET_RAW capabilities -- and deploying the
	// DaemonSet.
	BeforeAll(func() {
		By("creating manager namespace")
		cmd := exec.Command("kubectl", "create", "ns", namespace)
		_, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to create namespace")

		By("labeling the namespace to enforce the privileged security policy")
		cmd = exec.Command("kubectl", "label", "--overwrite", "ns", namespace,
			"pod-security.kubernetes.io/enforce=privileged")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to label namespace with privileged policy")

		By("retrieving the virtualip nodes with specific label")
		cmd = exec.Command("kubectl", "get", "nodes",
			"-l", "role=virtualip",
			"-o", "go-template={{ range .items }}"+
				"{{ .metadata.name }}"+
				"{{ \"\\n\" }}{{ end }}")
		nodeOutput, err := utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to get nodes")
		Expect(nodeOutput).NotTo(BeEmpty(), "No nodes with loadbalancer role found")
		nodeNames = utils.GetNonEmptyLines(nodeOutput)
		Expect(nodeNames).To(HaveLen(2), "expected 2 nodes with loadbalancer role")
		nodeName = nodeNames[0]
		otherNodeName = nodeNames[1]

		By("removing a secondary Docker network left behind by an earlier run")
		Expect(utils.RemoveNetworkIfPresent(secondaryNetworkName)).To(Succeed(),
			"Failed to remove the leftover secondary network")

		By("resolving the addresses the pools will declare from Kind's own network")
		kindNetworkName, kindSubnet, err = utils.GetKindNetwork(nodeNames[0])
		Expect(err).NotTo(HaveOccurred(), "Failed to resolve Kind's Docker network")

		By("creating a secondary Docker network")
		secondarySubnet, err = utils.CreateSecondaryNetwork(secondaryNetworkName)
		Expect(err).NotTo(HaveOccurred(), "Failed to create secondary network")

		By("attaching the virtualip nodes to the secondary Docker network")
		for _, node := range nodeNames {
			Expect(utils.ConnectNodeToNetwork(node, secondaryNetworkName)).To(Succeed(),
				"Failed to attach the node to the secondary network")
		}

		By("initializing the VirtualIP spec the DaemonSet mounts")
		Expect(initializeVirtualIPConfigMap()).To(Succeed())

		By("deploying the virtualip-manager")
		cmd = exec.Command("kubectl", "apply", "-k", "tests/e2e")
		_, err = utils.Run(cmd)
		Expect(err).NotTo(HaveOccurred(), "Failed to deploy the virtualip-manager")
	})

	// After all tests have been executed, clean up by undeploying the controller, uninstalling CRDs,
	// and deleting the namespace.
	AfterAll(func() {
		By("undeploying the virtualip-manager")
		cmd := exec.Command("kubectl", "delete", "-k", "tests/e2e")
		_, _ = utils.Run(cmd)

		By("removing the VirtualIP spec")
		cmd = exec.Command("kubectl", "delete", "configmap", virtualIPConfigMapName, "-n", namespace)
		_, _ = utils.Run(cmd)

		By("removing manager namespace")
		cmd = exec.Command("kubectl", "delete", "ns", namespace)
		_, _ = utils.Run(cmd)

		By("removing the secondary Docker network")
		_ = utils.RemoveNetworkIfPresent(secondaryNetworkName)
	})

	SetDefaultEventuallyTimeout(2 * time.Minute)
	SetDefaultEventuallyPollingInterval(time.Second)

	// applyAndRollout publishes the spec and cycles the DaemonSet so a fresh pod mounts it.
	// The entrypoint renders the configuration on startup, so once the rollout is done the
	// generated configuration -- and the keepalived running against it -- already reflect the
	// new spec.
	applyAndRollout := func(addresses []LocalizedVirtualIPAddress, healthcheck, healthcheckNodePort string) {
		GinkgoHelper()

		By("publishing the VirtualIP spec")
		Expect(applyVirtualIPConfigMap(addresses, healthcheck, healthcheckNodePort)).
			To(Succeed(), "Failed to publish the VirtualIP spec")

		By("restarting the virtualip-manager to refresh the configMap")
		_, err := utils.Run(exec.Command("kubectl", "rollout", "restart",
			"ds/virtualip-manager", "-n", namespace))
		Expect(err).NotTo(HaveOccurred(), "Failed to restart the virtualip-manager")

		By("waiting for the rollout to complete")
		_, err = utils.Run(exec.Command("kubectl", "rollout", "status",
			"ds/virtualip-manager", "-n", namespace, "--timeout=2m"))
		Expect(err).NotTo(HaveOccurred(), "The virtualip-manager rollout did not complete")
		Eventually(verifyManagerUp).Should(Succeed())
	}

	// crossNetworkAddresses allocates three VirtualIPs -- two on Kind's own network and one on
	// the secondary one -- owned by the named nodes, so a spec built from them makes the
	// manager resolve more than one interface of the node.
	crossNetworkAddresses := func(kindOwner, otherKindOwner, secondaryOwner string) []LocalizedVirtualIPAddress {
		GinkgoHelper()

		kindVirtualIPs, err := getAddressesOnNetwork(kindSubnet, kindNetworkName, 2)
		Expect(err).NotTo(HaveOccurred(), "Failed to generate VirtualIPs")
		secondaryVirtualIPs, err := getAddressesOnNetwork(secondarySubnet, secondaryNetworkName, 1)
		Expect(err).NotTo(HaveOccurred(), "Failed to generate VirtualIPs")

		return []LocalizedVirtualIPAddress{
			{VirtualIPAddress: kindVirtualIPs[0], Node: kindOwner},
			{VirtualIPAddress: kindVirtualIPs[1], Node: otherKindOwner},
			{VirtualIPAddress: secondaryVirtualIPs[0], Node: secondaryOwner},
		}
	}

	Context("Manager", func() {
		It("should run successfully", func() {
			By("validating that the virtualip-manager pod is running as expected")
			Eventually(verifyManagerUp).Should(Succeed())
		})

		It("should run keepalived as a non-root user with only the capabilities it needs", func() {
			Eventually(verifyManagerUp).Should(Succeed())

			for node, pod := range managerPods {
				By(fmt.Sprintf("checking that the entrypoint handed over to keepalived on %s", node))
				cmdline, err := execInManager(pod, "cat", "/proc/1/cmdline")
				Expect(err).NotTo(HaveOccurred(), "Failed to read the command line of PID 1")
				Expect(cmdline).To(ContainSubstring("keepalived"),
					"the entrypoint did not exec keepalived as PID 1")

				By("checking that keepalived does not run as root")
				uid, err := execInManager(pod, "id", "-u")
				Expect(err).NotTo(HaveOccurred(), "Failed to read the user of the manager process")
				Expect(strings.TrimSpace(uid)).NotTo(Equal("0"), "keepalived runs as root")

				By("checking the effective capabilities of the keepalived process")
				capabilities, err := effectiveCapabilities(pod)
				Expect(err).NotTo(HaveOccurred(), "Failed to read the capabilities of PID 1")
				Expect(capabilities).To(Equal(expectedCapabilities),
					"keepalived holds %#016x, the Dockerfile grants %#016x",
					capabilities, expectedCapabilities)
			}
		})
	})

	DescribeTable("When a configuration is deployed with VirtualIPs",
		func(healthcheck, healthcheckNodePort string, expectedScripts map[string]string) {
			addresses := crossNetworkAddresses(nodeName, otherNodeName, nodeName)
			applyAndRollout(addresses, healthcheck, healthcheckNodePort)

			// Both nodes mount the same spec and must render it differently, so the
			// configuration is read from each of them rather than from whichever pod the
			// API server happened to list first.
			for _, node := range nodeNames {
				By(fmt.Sprintf("checking the configuration generated on %s", node))
				generated, err := generatedConfiguration(managerPods[node])
				Expect(err).NotTo(HaveOccurred(), "Failed to read the generated configuration")
				parsed, err := parseKeepalivedConfig(generated)
				Expect(err).NotTo(HaveOccurred(), "Failed to parse the generated configuration")

				// expectedInstances resolves the interface and the MASTER/BACKUP state
				// independently of the manager, so comparing the whole slice already covers
				// the state, the priority, the interface and the virtual_ipaddress block of
				// every instance. The scripts are sorted because the template emits
				// check_get before check_get_nodeport.
				Expect(parsed.Instances).To(Equal(expectedInstances(
					node, addresses, slices.Sorted(maps.Keys(expectedScripts)))))
				Expect(parsed.Scripts).To(Equal(expectedScripts))
			}
		},
		Entry("and no healthcheck", "", "", map[string]string{}),
		Entry("and a standard healthcheck",
			"http://"+domain.NODE_IP_TOKEN+":8080/healthz", "",
			map[string]string{
				"check_get": "/etc/keepalived/check-get.sh http://" + managerNodeIP + ":8080/healthz",
			}),
		Entry("and a NodePort healthcheck",
			"", "http://localhost:31652",
			map[string]string{
				"check_get_nodeport": "/etc/keepalived/check-get.sh http://localhost:31652",
			}),
		Entry("and both standard and NodePort healthchecks",
			"http://"+domain.NODE_IP_TOKEN+":8080/healthz", "http://localhost:31652",
			map[string]string{
				"check_get":          "/etc/keepalived/check-get.sh http://" + managerNodeIP + ":8080/healthz",
				"check_get_nodeport": "/etc/keepalived/check-get.sh http://localhost:31652",
			}),
	)

	Context("When keepalived runs against the generated configuration", func() {
		It("should assign each VirtualIP to its owner node and to no other", func() {
			addresses := crossNetworkAddresses(nodeName, otherNodeName, nodeName)
			applyAndRollout(addresses, "", "")

			for _, address := range addresses {
				expectVirtualIPHeld(address.Node, address.Ip)
				expectVirtualIPReleased(peerOf(address.Node), address.Ip)
			}

			By("reaching the VirtualIPs from outside the nodes")
			// The host holds the gateway of every Docker bridge the suite uses, so a reply
			// here means keepalived both configured the address and answers ARP for it --
			// which is all a client of the VirtualIP depends on.
			if _, err := exec.LookPath("ping"); err != nil {
				Skip("ping is not available, cannot check the VirtualIPs from the host")
			}
			for _, address := range addresses {
				Eventually(func() error {
					_, err := utils.Run(exec.Command("ping", "-c", "1", "-W", "2", address.Ip))
					return err
				}).Should(Succeed(), "VirtualIP %s is not reachable from the host", address.Ip)
			}
		})

		It("should reassign VirtualIPs when a Node fails", func() {
			addresses := crossNetworkAddresses(nodeName, otherNodeName, nodeName)
			applyAndRollout(addresses, "", "")

			// The VirtualIPs of the node that is about to fail are the ones that have to
			// move; the one the surviving node owns has to stay where it is, which is what
			// tells a failover apart from keepalived restarting everything.
			failedOver := []LocalizedVirtualIPAddress{}
			for _, address := range addresses {
				if address.Node == nodeName {
					failedOver = append(failedOver, address)
				}
			}
			Expect(failedOver).NotTo(BeEmpty(), "the failing node owns no VirtualIP")

			By("waiting for every VirtualIP to settle on its owner")
			for _, address := range addresses {
				expectVirtualIPHeld(address.Node, address.Ip)
			}

			By(fmt.Sprintf("taking %s out of service", nodeName))
			DeferCleanup(func() {
				Expect(setVirtualIPRole(nodeName, true)).To(Succeed(),
					"Failed to put %s back in service", nodeName)
			})
			Expect(setVirtualIPRole(nodeName, false)).To(Succeed(),
				"Failed to take %s out of service", nodeName)
			expectManagerGone(nodeName)

			By(fmt.Sprintf("checking that %s released the VirtualIPs it owned", nodeName))
			// keepalived tears its addresses down when it is signalled, so the eviction
			// alone has to leave the node clean -- an address left behind here would be
			// claimed by two nodes at once.
			for _, address := range failedOver {
				expectVirtualIPReleased(nodeName, address.Ip)
			}

			By(fmt.Sprintf("checking that %s took the VirtualIPs over", otherNodeName))
			for _, address := range failedOver {
				expectVirtualIPHeld(otherNodeName, address.Ip)
			}

			By(fmt.Sprintf("putting %s back in service", nodeName))
			Expect(setVirtualIPRole(nodeName, true)).To(Succeed(),
				"Failed to put %s back in service", nodeName)
			// `rollout status` is no help here: until the DaemonSet controller has observed
			// the label, the rollout is complete with one pod. verifyManagerUp waits for a
			// ready pod on every labelled node, which is the state that matters.
			Eventually(verifyManagerUp).Should(Succeed())

			By(fmt.Sprintf("checking that %s preempts its VirtualIPs back", nodeName))
			// The recovered node comes back as MASTER with priority 130 against the peer's
			// 80, so it has to preempt -- and the peer has to step down, not keep the
			// address it took over.
			for _, address := range failedOver {
				expectVirtualIPHeld(nodeName, address.Ip)
				expectVirtualIPReleased(otherNodeName, address.Ip)
			}

			By(fmt.Sprintf("checking that the VirtualIPs of %s never moved", otherNodeName))
			for _, address := range addresses {
				if address.Node != otherNodeName {
					continue
				}
				expectVirtualIPHeld(otherNodeName, address.Ip)
				expectVirtualIPReleased(nodeName, address.Ip)
			}
		})
	})
})

// peerOf returns the labelled node that is not node, i.e. the BACKUP for every VirtualIP node
// owns. The suite runs on exactly two labelled nodes, which BeforeAll asserts.
func peerOf(node string) string {
	if node == nodeName {
		return otherNodeName
	}
	return nodeName
}

// expectVirtualIPHeld waits for node to carry address as a /32 on the interface whose subnet
// contains it. The interface is resolved through the node container, independently of what the
// manager rendered, so the assertion holds against the real host network stack.
func expectVirtualIPHeld(node, address string) {
	GinkgoHelper()

	iface, err := nodeInterface(node, address)
	Expect(err).NotTo(HaveOccurred(), "Failed to resolve the interface for %s on %s", address, node)

	By(fmt.Sprintf("waiting for %s to be assigned to %s on %s", address, iface, node))
	Eventually(func() ([]string, error) {
		return nodeAddressesOnInterface(node, iface)
	}).Should(ContainElement(address+"/32"),
		"keepalived did not assign %s to %s on %s", address, iface, node)
}

// expectVirtualIPReleased waits for node to let address go, then checks it stays away. A BACKUP
// promotes itself while it has heard no advertisement, so it may legitimately hold the address
// for a moment before the MASTER preempts it; what has to hold is that it does not keep it.
func expectVirtualIPReleased(node, address string) {
	GinkgoHelper()

	iface, err := nodeInterface(node, address)
	Expect(err).NotTo(HaveOccurred(), "Failed to resolve the interface for %s on %s", address, node)

	By(fmt.Sprintf("checking that %s does not claim %s", node, address))
	Eventually(func() ([]string, error) {
		return nodeAddressesOnInterface(node, iface)
	}).ShouldNot(ContainElement(address + "/32"))
	Consistently(func() ([]string, error) {
		return nodeAddressesOnInterface(node, iface)
	}, 5*time.Second, time.Second).ShouldNot(ContainElement(address+"/32"),
		"node %s claimed %s", node, address)
}

// setVirtualIPRole adds or removes the role=virtualip label the DaemonSet selects on, which is
// how the suite takes a node out of service: without the label the pod is evicted and is not
// rescheduled until the label comes back. A Kind node cannot be stopped instead -- the node
// container is also where the VirtualIPs are read from -- and deleting the pod is immediately
// undone by the DaemonSet, leaving no window for the peer to take over.
func setVirtualIPRole(node string, inService bool) error {
	label := "role-"
	if inService {
		label = "role=virtualip"
	}
	_, err := utils.Run(exec.Command("kubectl", "label", "--overwrite", "node", node, label))
	return err
}

// expectManagerGone waits until no virtualip-manager pod is left on node, so the assertions that
// follow cannot pass against a keepalived that is still running.
func expectManagerGone(node string) {
	GinkgoHelper()

	Eventually(func() (string, error) {
		return utils.Run(exec.Command("kubectl", "get", "pods",
			"-l", "app.kubernetes.io/name=virtualip-manager",
			"--field-selector", "spec.nodeName="+node,
			"-n", namespace, "-o", "name"))
	}).Should(BeEmpty(), "the virtualip-manager pod on %s was not evicted", node)
}

// initializeVirtualIPConfigMap seeds a valid spec before the DaemonSet is created. The
// entrypoint renders the configuration before it execs keepalived, so a spec with no address
// would fail validation and leave the pod crash-looping instead of Running.
func initializeVirtualIPConfigMap() error {
	addresses, err := getAddressesOnNetwork(kindSubnet, kindNetworkName, 1)
	if err != nil {
		return err
	}

	return applyVirtualIPConfigMap([]LocalizedVirtualIPAddress{
		{VirtualIPAddress: addresses[0], Node: nodeName},
	}, "", "")
}

func generateConfiguration(virtualIPs []LocalizedVirtualIPAddress, healthcheck string, healthcheckNodePort string) (string, error) {
	if len(nodeNames) == 0 && len(virtualIPs) != 0 {
		return "", fmt.Errorf("no nodes to assign the VirtualIPs to")
	}

	spec := &strings.Builder{}
	spec.WriteString("apiVersion: loadbalancer.scality.com/v1alpha1\n")
	spec.WriteString("kind: VirtualIPConfiguration\n")
	spec.WriteString("addresses:\n")
	for _, address := range virtualIPs {
		fmt.Fprintf(spec, "- ip: %s\n  node: %s\n  vrId: %s\n",
			address.Ip, address.Node, address.VrId)
	}
	if healthcheck != "" {
		fmt.Fprintf(spec, "healthcheck: %s\n", healthcheck)
	}
	if healthcheckNodePort != "" {
		fmt.Fprintf(spec, "healthcheckNodePort: %s\n", healthcheckNodePort)
	}

	return spec.String(), nil
}

// applyVirtualIPConfigMap publishes the VirtualIPConfiguration the DaemonSet mounts at
// /etc/keepalived/keepalived-input.yaml. The addresses are only known once the suite has
// read Kind's subnet, so the spec is built here rather than checked in as a fixture; the
// VIPs are spread over the eligible nodes so each one owns (is MASTER for) a share.
func applyVirtualIPConfigMap(addresses []LocalizedVirtualIPAddress, healthcheck string, healthcheckNodePort string) error {
	spec, err := generateConfiguration(addresses, healthcheck, healthcheckNodePort)
	if err != nil {
		return err
	}
	cmd := exec.Command("kubectl", "create", "configmap", virtualIPConfigMapName,
		"-n", namespace,
		"--from-file=keepalived-input.yaml=/dev/stdin",
		"--dry-run=client", "-o", "yaml")
	cmd.Stdin = strings.NewReader(spec)
	manifest, err := utils.Run(cmd)
	if err != nil {
		return err
	}

	apply := exec.Command("kubectl", "apply", "-f", "-")
	apply.Stdin = strings.NewReader(manifest)
	_, err = utils.Run(apply)
	return err
}

// getAddressesOnNetwork returns the count highest usable addresses of subnet, ascending. The VIPs
// are taken from the top because Docker's IPAM allocates from the bottom of the subnet:
// on Kind's own network the suite cannot fence IPAM off with --ip-range, so it stays as
// far away from it as the subnet allows.
func getAddressesOnNetwork(subnet *net.IPNet, kindNetwork string, count int) ([]VirtualIPAddress, error) {
	network := subnet.IP.To4()
	mask := net.IP(subnet.Mask).To4()
	if network == nil || mask == nil {
		return nil, fmt.Errorf("subnet %s is not IPv4", subnet)
	}
	broadcast := binary.BigEndian.Uint32(network) | ^binary.BigEndian.Uint32(mask)

	addresses := []VirtualIPAddress{}
	// Walk down from the broadcast address, which is itself skipped.
	for offset := uint32(count); offset >= 1; offset-- {
		address := make(net.IP, net.IPv4len)
		binary.BigEndian.PutUint32(address, broadcast-offset)
		if !subnet.Contains(address) {
			return nil, fmt.Errorf("subnet %s is too small for %d VirtualIPs", subnet, count)
		}

		addresses = append(addresses, VirtualIPAddress{
			Ip:   address.String(),
			VrId: nextVrId(),
		})
	}

	// check if the addresses are free
	free, err := assertAddressesFree(kindNetwork, addresses)
	if err != nil {
		return nil, err
	}
	if !free {
		return nil, fmt.Errorf("addresses are not free")
	}
	return addresses, nil
}

// nextVrId hands out the next VRRP virtual_router_id. Deriving the id from the address's
// offset in its subnet collides as soon as two networks contribute an address at the same
// offset, so the ids are allocated from a single counter instead. VRRP ids are 8 bit and
// 0 is reserved.
func nextVrId() string {
	lastVrId++
	Expect(lastVrId).To(BeNumerically("<=", 255), "exhausted the VRRP virtual_router_id range")
	return fmt.Sprintf("%d", lastVrId)
}

// assertAddressesFree returns false when a container already holds one of the addresses the suite
// is about to advertise. It cannot see a VIP another suite run advertises -- that address
// belongs to no container -- so it catches the common collision, not every one.
func assertAddressesFree(network string, addresses []VirtualIPAddress) (bool, error) {
	output, err := utils.Run(exec.Command("docker", "network", "inspect", network, "-f",
		`{{range .Containers}}{{.Name}}={{.IPv4Address}} {{end}}`))
	if err != nil {
		return false, err
	}

	for _, container := range strings.Fields(output) {
		_, cidr, found := strings.Cut(container, "=")
		if !found {
			continue
		}
		ip, _, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if slices.ContainsFunc(addresses, func(vip VirtualIPAddress) bool {
			return vip.Ip == ip.String()
		}) {
			return false, nil
		}
	}
	return true, nil
}

// nodeInterface returns the name of the interface on node whose configured subnet contains
// address. Kind names each node's container after the node, and the VIPs live in the host
// network namespace, not in any pod's -- so the container is where to look. This mirrors what
// generate-config resolves, computed independently of it.
func nodeInterface(node, address string) (string, error) {
	target := net.ParseIP(address)
	if target == nil {
		return "", fmt.Errorf("%q is not an IP address", address)
	}

	cmd := exec.Command("docker", "exec", node, "ip", "-4", "-oneline", "address", "show")
	output, err := utils.Run(cmd)
	if err != nil {
		return "", err
	}

	for _, line := range utils.GetNonEmptyLines(output) {
		// "2: eth1    inet 10.0.0.11/24 brd ... scope global eth1"
		fields := strings.Fields(line)
		for i, field := range fields {
			if field != "inet" || i+1 >= len(fields) {
				continue
			}
			_, subnet, err := net.ParseCIDR(fields[i+1])
			if err != nil {
				return "", fmt.Errorf("parsing address %q on node %s: %w", fields[i+1], node, err)
			}
			if subnet.Contains(target) {
				return fields[1], nil
			}
		}
	}
	return "", fmt.Errorf("no interface on node %s carries the subnet of %s", node, address)
}

// VirtualIPAddress is a simplified representation of a VirtualIPAddress.
type VirtualIPAddress struct {
	Ip   string
	VrId string
}

// LocalizedVirtualIPAddress is a VirtualIPAddress with a node name.
type LocalizedVirtualIPAddress struct {
	VirtualIPAddress
	Node string
}

// execInManager runs a command inside the running manager pod.
func execInManager(pod string, command ...string) (string, error) {
	args := append([]string{"exec", pod, "-n", namespace, "--"}, command...)
	return utils.Run(exec.Command("kubectl", args...))
}

// generatedConfiguration returns the configuration the entrypoint rendered before it handed
// over to keepalived, which is the one keepalived is running against.
func generatedConfiguration(pod string) (string, error) {
	return execInManager(pod, "cat", generatedConfigPath)
}

// effectiveCapabilities returns the capability set PID 1 of the manager pod -- keepalived --
// actually runs with. The image drops libcap after the setcap call, so the capabilities are
// read from procfs rather than from the file they are attached to.
func effectiveCapabilities(pod string) (uint64, error) {
	status, err := execInManager(pod, "cat", "/proc/1/status")
	if err != nil {
		return 0, err
	}

	for _, line := range utils.GetNonEmptyLines(status) {
		value, found := strings.CutPrefix(strings.TrimSpace(line), "CapEff:")
		if !found {
			continue
		}
		return strconv.ParseUint(strings.TrimSpace(value), 16, 64)
	}
	return 0, fmt.Errorf("no CapEff line in the status of PID 1")
}

// nodeAddressesOnInterface returns the IPv4 addresses configured on iface of node, in CIDR
// form. The VirtualIPs live in the node's network namespace, not in any pod's, so the node
// container is where to look for them.
func nodeAddressesOnInterface(node, iface string) ([]string, error) {
	output, err := utils.Run(exec.Command("docker", "exec", node,
		"ip", "-4", "-oneline", "address", "show", "dev", iface))
	if err != nil {
		return nil, err
	}

	addresses := []string{}
	for _, line := range utils.GetNonEmptyLines(output) {
		fields := strings.Fields(line)
		for index, field := range fields {
			if field == "inet" && index+1 < len(fields) {
				addresses = append(addresses, fields[index+1])
			}
		}
	}
	return addresses, nil
}

// vrrpInstance is the parsed form of a `vrrp_instance` block of a keepalived configuration.
type vrrpInstance struct {
	Name               string
	State              string
	Interface          string
	Priority           string
	VirtualRouterID    string
	VirtualIPAddresses []string
	TrackScripts       []string
}

// keepalivedConfig holds the parts of a keepalived configuration the suite asserts on.
type keepalivedConfig struct {
	Instances []vrrpInstance
	// Scripts maps a vrrp_script name to its `script` argument, unquoted.
	Scripts map[string]string
}

// parseKeepalivedConfig reads the generated configuration into its meaning rather than
// comparing it to a golden file: what e2e adds over the unit tests is that the interface and
// the MASTER/BACKUP state are resolved against the real host network stack and NODE_NAME, and
// a golden file would only re-state the template's layout. Unknown directives are rejected so
// a template addition shows up here instead of being silently ignored.
func parseKeepalivedConfig(config string) (keepalivedConfig, error) {
	parsed := keepalivedConfig{Scripts: map[string]string{}}
	current, script, section := -1, "", ""

	for _, raw := range strings.Split(config, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)

		switch {
		case line == "}":
			if section != "" {
				section = ""
			} else {
				current, script = -1, ""
			}
		case fields[0] == "vrrp_instance" && len(fields) == 3:
			parsed.Instances = append(parsed.Instances, vrrpInstance{Name: fields[1]})
			current = len(parsed.Instances) - 1
		case fields[0] == "vrrp_script" && len(fields) == 3:
			script = fields[1]
		case strings.HasSuffix(line, "{"):
			section = fields[0] // global_defs, virtual_ipaddress, track_script, ...
		case script != "" && fields[0] == "script":
			parsed.Scripts[script] = strings.Trim(strings.TrimPrefix(line, "script "), `"`)
		case current < 0:
			continue
		case section == "virtual_ipaddress":
			parsed.Instances[current].VirtualIPAddresses = append(
				parsed.Instances[current].VirtualIPAddresses, fields[0])
		case section == "track_script":
			parsed.Instances[current].TrackScripts = append(
				parsed.Instances[current].TrackScripts, fields[0])
		case len(fields) != 2:
			return parsed, fmt.Errorf("unexpected line %q in vrrp_instance %s",
				line, parsed.Instances[current].Name)
		default:
			instance := &parsed.Instances[current]
			switch fields[0] {
			case "state":
				instance.State = fields[1]
			case "interface":
				instance.Interface = fields[1]
			case "priority":
				instance.Priority = fields[1]
			case "virtual_router_id":
				instance.VirtualRouterID = fields[1]
			default:
				return parsed, fmt.Errorf("unknown key %q in vrrp_instance %s", fields[0], instance.Name)
			}
		}
	}
	if current >= 0 || script != "" || section != "" {
		return parsed, fmt.Errorf("unterminated block in the generated configuration")
	}
	return parsed, nil
}

// expectedInstances derives the vrrp_instance blocks the manager must generate for addresses
// from the point of view of node: the blocks keep the spec's order (VI_1..VI_n), the node named
// by an entry is that VIP's MASTER and every other node is a BACKUP, and the interface is the
// one that actually carries the VIP's subnet on that node. The same spec therefore renders
// differently on each node, which is what makes it worth reading the configuration of both.
func expectedInstances(node string, addresses []LocalizedVirtualIPAddress,
	trackScripts []string,
) []vrrpInstance {
	instances := make([]vrrpInstance, 0, len(addresses))
	for index, address := range addresses {
		iface, err := nodeInterface(node, address.Ip)
		Expect(err).NotTo(HaveOccurred(), "Failed to resolve the interface for %s", address.Ip)

		state, priority := "BACKUP", "80"
		if address.Node == node {
			state, priority = "MASTER", "130"
		}
		instances = append(instances, vrrpInstance{
			Name:               fmt.Sprintf("VI_%d", index+1),
			State:              state,
			Interface:          iface,
			Priority:           priority,
			VirtualRouterID:    address.VrId,
			VirtualIPAddresses: []string{address.Ip},
			TrackScripts:       trackScripts,
		})
	}
	return instances
}
