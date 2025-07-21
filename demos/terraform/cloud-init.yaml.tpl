#cloud-config
hostname: ${hostname}
package_update: true
package_upgrade: true

packages:
  - snapd

write_files:
  - path: /etc/netplan/99-microcloud.yaml
    content: |
      network:
        version: 2
        ethernets:
          ${ovn_uplink_interface}:
            accept-ra: false
            dhcp4: false
            link-local: []
    permissions: '0600'

runcmd:
  - |
    /usr/sbin/netplan apply && \
    /usr/bin/snap install lxd --channel=${lxd_channel} --cohort="+" && \
    /usr/bin/snap install microceph --channel=${microceph_channel} --cohort="+" && \
    /usr/bin/snap install microovn --channel=${microovn_channel} --cohort="+" && \
    /usr/bin/snap install microcloud --channel=${microcloud_channel} --cohort="+" && \
    /snap/bin/microcloud preseed < /root/microcloud_preseed.yaml
