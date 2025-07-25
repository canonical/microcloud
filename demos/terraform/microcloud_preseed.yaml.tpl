initiator: ${initiator}
lookup_subnet: ${subnet}
lookup_timeout: ${lookup_timeout}

session_passphrase: "${session_passphrase}"
session_timeout: ${session_timeout}

systems:
%{ for system in systems ~}
- name: ${system.name}
  ovn_uplink_interface: ${ovn_uplink_interface}
  ovn_underlay_ip: ${system.ip}
  storage:
    local:
      path: ${local_disk_device}
      wipe: true
%{ if system.has_ceph ~}
    ceph:
      - path: ${ceph_disk_device}
        wipe: true
        encrypt: false
%{ endif ~}
%{ endfor ~}

%{ if current_node_has_ceph ~}
ceph:
  cephfs: true
  internal_network: ${internal_subnet}
  public_network: ${subnet}
%{ endif ~}

ovn:
  ipv4_gateway: ${ovn_ipv4_gateway}
  ipv4_range: ${ovn_ipv4_range}
  ipv6_gateway: ${ovn_ipv6_gateway}
  dns_servers: ${ovn_dns_servers}
