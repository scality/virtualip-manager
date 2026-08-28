package integration

var outputComplete = `global_defs {
    enable_script_security
    script_user keepalived keepalived
}

vrrp_script check_get {
    script "/etc/keepalived/check-get.sh https://1.1.1.1:443/healthz"
    interval 5
    weight 60
}

vrrp_instance VI_1 {
    state MASTER
    interface eth0
    priority 130
    virtual_router_id 51
    virtual_ipaddress {
        172.17.0.15
    }
    track_script {
        check_get
    }
}

vrrp_instance VI_2 {
    state BACKUP
    interface eth1
    priority 80
    virtual_router_id 52
    virtual_ipaddress {
        172.17.0.16
    }
    track_script {
        check_get
    }
}

vrrp_instance VI_3 {
    state BACKUP
    interface eth2
    priority 80
    virtual_router_id 53
    virtual_ipaddress {
        172.17.0.17
    }
    track_script {
        check_get
    }
}
`

var outputNoHealthcheck = `global_defs {
    enable_script_security
    script_user keepalived keepalived
}

vrrp_instance VI_1 {
    state MASTER
    interface eth0
    priority 130
    virtual_router_id 51
    virtual_ipaddress {
        172.17.0.15
    }
}

vrrp_instance VI_2 {
    state BACKUP
    interface eth1
    priority 80
    virtual_router_id 52
    virtual_ipaddress {
        172.17.0.16
    }
}

vrrp_instance VI_3 {
    state BACKUP
    interface eth2
    priority 80
    virtual_router_id 53
    virtual_ipaddress {
        172.17.0.17
    }
}
`

var outputNodePortOnly = `global_defs {
    enable_script_security
    script_user keepalived keepalived
}

vrrp_script check_get_nodeport {
    script "/etc/keepalived/check-get.sh http://localhost:31846"
    interval 5
    weight 60
}

vrrp_instance VI_1 {
    state MASTER
    interface eth0
    priority 130
    virtual_router_id 51
    virtual_ipaddress {
        172.17.0.15
    }
    track_script {
        check_get_nodeport
    }
}

vrrp_instance VI_2 {
    state BACKUP
    interface eth1
    priority 80
    virtual_router_id 52
    virtual_ipaddress {
        172.17.0.16
    }
    track_script {
        check_get_nodeport
    }
}

vrrp_instance VI_3 {
    state BACKUP
    interface eth2
    priority 80
    virtual_router_id 53
    virtual_ipaddress {
        172.17.0.17
    }
    track_script {
        check_get_nodeport
    }
}
`

var outputBothHealthchecks = `global_defs {
    enable_script_security
    script_user keepalived keepalived
}

vrrp_script check_get {
    script "/etc/keepalived/check-get.sh https://1.1.1.1:443/healthz"
    interval 5
    weight 60
}

vrrp_script check_get_nodeport {
    script "/etc/keepalived/check-get.sh http://localhost:31846"
    interval 5
    weight 60
}

vrrp_instance VI_1 {
    state MASTER
    interface eth0
    priority 130
    virtual_router_id 51
    virtual_ipaddress {
        172.17.0.15
    }
    track_script {
        check_get
        check_get_nodeport
    }
}

vrrp_instance VI_2 {
    state BACKUP
    interface eth1
    priority 80
    virtual_router_id 52
    virtual_ipaddress {
        172.17.0.16
    }
    track_script {
        check_get
        check_get_nodeport
    }
}

vrrp_instance VI_3 {
    state BACKUP
    interface eth2
    priority 80
    virtual_router_id 53
    virtual_ipaddress {
        172.17.0.17
    }
    track_script {
        check_get
        check_get_nodeport
    }
}
`
