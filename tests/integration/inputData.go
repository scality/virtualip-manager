package integration

var inputComplete = []byte(`---
apiVersion: loadbalancer.scality.com/v1alpha1
kind: VirtualIPConfiguration
addresses:
- ip: 172.17.0.15
  node: bootstrap
  vrId: 51
- ip: 172.17.0.16
  node: node1
  vrId: 52
- ip: 172.17.0.17
  node: node2
  vrId: 53
healthcheck: https://__NODE_IP__:443/healthz
`)

var inputNoHealthcheck = []byte(`---
apiVersion: loadbalancer.scality.com/v1alpha1
kind: VirtualIPConfiguration
addresses:
- ip: 172.17.0.15
  node: bootstrap
  vrId: 51
- ip: 172.17.0.16
  node: node1
  vrId: 52
- ip: 172.17.0.17
  node: node2
  vrId: 53
`)

var inputEmptyHealthcheck = []byte(`---
apiVersion: loadbalancer.scality.com/v1alpha1
kind: VirtualIPConfiguration
addresses:
- ip: 172.17.0.15
  node: bootstrap
  vrId: 51
- ip: 172.17.0.16
  node: node1
  vrId: 52
- ip: 172.17.0.17
  node: node2
  vrId: 53
healthcheck:
`)

var inputMalformed = []byte(`---
apiVersion: loadbalancer.scality.com/v1alpha1
kind: VirtualIPConfiguration
addresses:
- ip: 172.17.0.15
  node: bootstrap
  vrId: "51"
healthcheck: https://__NODE_IP__:443/healthz
`)

var inputEmptyAddresses = []byte(`---
apiVersion: loadbalancer.scality.com/v1alpha1
kind: VirtualIPConfiguration
addresses: []
healthcheck: https://127.0.0.1:443/healthz
`)

var inputMissingAddresses = []byte(`---
apiVersion: loadbalancer.scality.com/v1alpha1
kind: VirtualIPConfiguration
healthcheck: https://127.0.0.1:443/healthz
`)

var inputMissingKind = []byte(`---
apiVersion: loadbalancer.scality.com/v1alpha1
addresses:
- ip: 172.17.0.15
  node: bootstrap
  vrId: 51
`)

var inputWrongKind = []byte(`
apiVersion: loadbalancer.scality.com/v1alpha1
kind: WrongKind
addresses:
- ip: 172.17.0.15
  node: bootstrap
  vrId: 51
`)

var inputMissingApiVersion = []byte(`---
kind: VirtualIPConfiguration
addresses:
- ip: 172.17.0.15
  node: bootstrap
  vrId: 51
healthcheck: https://127.0.0.1:443/healthz
`)

var inputWrongApiVersion = []byte(`---
kind: VirtualIPConfiguration
apiVersion: wrong.scality.com/v1alpha1
addresses:
- ip: 172.17.0.15
  node: bootstrap
  vrId: 51
`)

var inputEmpty = []byte(``)
var inputCommentOnly = []byte(`# This is a comment`)

var inputNoMatchingInterfaces = []byte(`---
apiVersion: loadbalancer.scality.com/v1alpha1
kind: VirtualIPConfiguration
addresses:
- ip: 10.50.0.1
  node: bootstrap
  vrId: 51
`)
