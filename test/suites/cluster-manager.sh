#!/bin/bash

test_cluster_manager() {
  # Reset the systems to a single node
  reset_systems 1 0 0

  export MULTI_NODE="no"
  export LOOKUP_IFACE="enp5s0"
  export OVN_WARNING="yes"
  export SKIP_LOOKUP=1
  join_session init micro01

  lxc exec micro01 -- tail -1 out | grep "MicroCloud is ready" -q

  echo "==> Test cluster manager without previous setup"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager show 2>&1 | grep "Error: Cluster manager not found" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager delete 2>&1 | grep "Error: Cluster manager not found" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager get update-interval-seconds 2>&1 | grep "Error: Cluster manager not found" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager set update-interval-seconds 10 2>&1 | grep "Error: Cluster manager not found" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager unset update-interval-seconds 2>&1 | grep "Error: Cluster manager not found" -q

  echo "==> Create a cert for dummy cluster manager"
  lxc exec micro01 -- openssl req -x509 -newkey rsa:2048 -nodes -keyout key.pem -out cert.pem -days 1 -subj "/CN=localhost" -addext "subjectAltName = DNS:localhost"
  echo "==> Start dummy cluster manager server"
  lxc exec micro01 -- sh -c "nohup sh -c '(
    while true; do
      printf \"HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n\"
      sleep 1
    done
  ) | openssl s_server -accept 3000 -key key.pem -cert cert.pem -quiet > /tmp/openssl_server.log 2>&1 &'" &
  echo "==> Create a token for connecting to cluster manager"
  fingerprint=$(lxc exec micro01 -- openssl x509 -in cert.pem -noout -fingerprint -sha256 | cut -d'=' -f2 | tr -d ':' | tr 'A-F' 'a-f')
  token=$(echo '{"secret":"not_so_secret","expires_at":"2125-04-10T12:32:00Z","addresses":["localhost:3000"],"server_name":"localhost","fingerprint":"'"$fingerprint"'"}' | base64 -w0)

  # Sleep some time so the dummy server is up and running
  sleep 3

  echo "==> Join the dummy cluster manager"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager join "$token" | grep "Successfully joined cluster manager" -q

  echo "==> Run cluster manager commands"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager get update-interval-seconds | grep "60" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager set update-interval-seconds 15
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager get update-interval-seconds | grep "15" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager unset update-interval-seconds
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager set update-interval-seconds 60
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager show | grep "certificate_fingerprint:" -q

  echo "==> Delete cluster manager"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager delete
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager show 2>&1 | grep "Error: Cluster manager not found" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager join "$token" | grep "Successfully joined cluster manager" -q

  echo "==> Stop dummy cluster manager"
  lxc exec micro01 -- sh -c "pgrep -f 'openssl s_server -accept 3000' | xargs kill" || true

  echo "==> Delete cluster manager with force flag"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager delete 2>&1 | grep "Unable to connect to: localhost:3000" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager delete --force
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager show 2>&1 | grep "Error: Cluster manager not found" -q
}
