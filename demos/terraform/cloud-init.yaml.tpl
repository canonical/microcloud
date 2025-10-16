#cloud-config

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
    # Workaround for https://bugs.launchpad.net/snapd/+bug/2104066
    (
      /usr/bin/mkdir -p /etc/systemd/system/snapd.service.d && \
      /usr/bin/printf '%s\n%s\n%s\n' '# Workaround for https://bugs.launchpad.net/snapd/+bug/2104066' '[Service]' 'Environment=SNAPD_STANDBY_WAIT=1m' > /etc/systemd/system/snapd.service.d/override.conf && \
      /usr/bin/systemctl daemon-reload && \
      /usr/bin/systemctl try-restart snapd.service
    ) || echo "snapd workaround failed, continuing with snap installations" && \
    /usr/bin/snap install lxd --channel=${lxd_channel} --cohort="+" && \
    /usr/bin/snap install microceph --channel=${microceph_channel} --cohort="+" && \
    /usr/bin/snap install microovn --channel=${microovn_channel} --cohort="+" && \
    /usr/bin/snap install microcloud --channel=${microcloud_channel} --cohort="+" && \
    /snap/bin/microcloud preseed < /root/microcloud_preseed.yaml
