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
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager get update_interval_seconds 2>&1 | grep "Error: Cluster manager not found" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager set update_interval_seconds 10 2>&1 | grep "Error: Cluster manager not found" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager unset update_interval_seconds 2>&1 | grep "Error: Cluster manager not found" -q

  echo "==> Create a cert for dummy cluster manager"
  lxc exec micro01 -- openssl req -x509 -newkey rsa:2048 -nodes -keyout key.pem -out cert.pem -days 1 -subj "/CN=localhost" -addext "subjectAltName = DNS:localhost"
  echo "==> Start dummy cluster manager server"
  lxc exec micro01 -- sh -c "cat > ws_server.py << 'PYEOF'
import ssl, socket, hashlib, base64

WS_MAGIC = '258EAFA5-E914-47DA-95CA-C5AB0DC85B11'
HIT_FILE = 'ws_hit'

ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
ctx.load_cert_chain('cert.pem', 'key.pem')

srv = socket.socket(socket.AF_INET, socket.SOCK_STREAM)
srv.setsockopt(socket.SOL_SOCKET, socket.SO_REUSEADDR, 1)
srv.bind(('127.0.0.1', 3000))
srv.listen(5)

while True:
    raw, _ = srv.accept()
    conn = ctx.wrap_socket(raw, server_side=True)
    data = b''
    while b'\r\n\r\n' not in data:
        data += conn.recv(4096)
    headers = {}
    lines = data.split(b'\r\n')
    request_line = lines[0].decode()
    for line in lines[1:]:
        if b':' in line:
            k, v = line.split(b':', 1)
            headers[k.strip().lower()] = v.strip()
    path = request_line.split(' ')[1] if ' ' in request_line else ''
    if headers.get(b'upgrade', b'').lower() == b'websocket' and path == '/1.0/remote-cluster/ws':
        key = headers.get(b'sec-websocket-key', b'').decode()
        accept = base64.b64encode(hashlib.sha1((key + WS_MAGIC).encode()).digest()).decode()
        response = (
            'HTTP/1.1 101 Switching Protocols\r\n'
            'Upgrade: websocket\r\n'
            'Connection: Upgrade\r\n'
            f'Sec-WebSocket-Accept: {accept}\r\n\r\n'
        )
        conn.sendall(response.encode())
        open(HIT_FILE, 'w').write('hit')
        import time; time.sleep(30) # keeps the WebSocket connection open. avoid server to immediately close the connection.
    else:
        conn.sendall(b'HTTP/1.1 200 OK\r\nContent-Length: 0\r\n\r\n')
        import time; time.sleep(1)
    conn.close()
PYEOF"
  lxc exec micro01 -- sh -c "nohup sh -c 'nohup python3 ws_server.py > ws_server.log 2>&1'" &

  echo "==> Create a token for connecting to cluster manager"
  fingerprint=$(lxc exec micro01 -- openssl x509 -in cert.pem -noout -fingerprint -sha256 | cut -d'=' -f2 | tr -d ':' | tr 'A-F' 'a-f')
  token=$(echo '{"secret":"not_so_secret","expires_at":"2125-04-10T12:32:00Z","addresses":["localhost:3000"],"server_name":"localhost","fingerprint":"'"$fingerprint"'"}' | base64 -w0)

  # Sleep some time so the dummy server is up and running
  sleep 3

  echo "==> Join the dummy cluster manager"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager join "$token" | grep "Successfully joined cluster manager" -q

  echo "==> Run cluster manager commands"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager get update_interval_seconds | grep "60" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager set update_interval_seconds 15
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager get update_interval_seconds | grep "15" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager unset update_interval_seconds
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager set update_interval_seconds 60
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager show | grep "certificate_fingerprint:" -q

  echo "==> Delete cluster manager"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager delete
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager show 2>&1 | grep "Error: Cluster manager not found" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager join "$token" | grep "Successfully joined cluster manager" -q

  echo "==> Tunnel config for cluster manager"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager set reverse_tunnel true
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager get reverse_tunnel | grep "true" -q

  echo "==> Wait for daemon to open WebSocket tunnel to /1.0/remote-cluster/ws"
  # The reconcile ticker fires every 10 s; wait up to 25 s
  for _i in $(seq 1 25); do
    if lxc exec micro01 -- test -f ws_hit 2>/dev/null; then
      break
    fi
    sleep 1
  done
  if ! lxc exec micro01 -- test -f ws_hit; then
    echo "ERROR: WebSocket tunnel was never opened on /1.0/remote-cluster/ws"
    exit 1
  fi

  echo "==> WebSocket tunnel confirmed on /1.0/remote-cluster/ws"

  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager unset reverse_tunnel
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager get reverse_tunnel | grep "false" -q

  echo "==> Stop WebSocket dummy cluster manager server"
  lxc exec micro01 -- sh -c "pgrep -f ws_server.py | xargs kill" || true

  echo "==> Delete cluster manager with force flag"
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager delete 2>&1 | grep "Cannot connect to: localhost:3000" -q
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager delete --force
  lxc exec micro01 --env TEST_CONSOLE=0 -- microcloud cluster-manager show 2>&1 | grep "Error: Cluster manager not found" -q
}
